package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"zaee-engine/pkg/resilience"
	"zaee-engine/pkg/schema"
)

type Processor struct {
	Registry *schema.Registry
	Resil    *resilience.Resilience
}

func NewProcessor(registry *schema.Registry, resil *resilience.Resilience) *Processor {
	return &Processor{
		Registry: registry,
		Resil:    resil,
	}
}

// Evaluate evaluates the fields against SDT, adds tier/flags, and returns the augmented JSON.
func (p *Processor) Evaluate(sensorID string, payloadTime time.Time, fieldsMap map[string]interface{}, inferResult schema.InferResult) ([]byte, error) {
	sensorSchema, exists := p.Registry.GetSensor(sensorID)
	tier := "standard"
	coldStartActive := false
	if exists {
		tier = sensorSchema.Tier
		coldStartActive = sensorSchema.Checklist.Active
	}

	if exists {
		sensorSchema.LastSeenTime = time.Now()
		if sensorSchema.DropoutFlagged {
			sensorSchema.DropoutFlagged = false
		}
	}

	// Delete SDT state for any fields that were promoted due to schema drift
	for _, fieldName := range inferResult.DriftPromotions {
		if p.Resil != nil {
			p.Resil.DeleteSDTStateSafe(sensorID, fieldName)
		}
	}

	// PHASE 4: Swinging Door Trending (SDT) & Heartbeat
	var fieldsToKeep = make(map[string]interface{})
	shouldEmit := false
	useArchivedTime := false
	var archivedTime time.Time

	heartbeatTriggered := false
	if !coldStartActive && exists {
		if sensorSchema.LastHeartbeatTime.IsZero() {
			sensorSchema.LastHeartbeatTime = time.Now()
		} else if time.Since(sensorSchema.LastHeartbeatTime) > sensorSchema.HeartbeatInterval {
			heartbeatTriggered = true
		}
	}

	if coldStartActive || tier == "critical" || tier == "flagged" {
		// Bypass SDT entirely
		shouldEmit = true
		fieldsToKeep = fieldsMap
		if exists {
			sensorSchema.LastHeartbeatTime = time.Now()
		}
	} else if exists {
		for k, v := range fieldsMap {
			f, ok := sensorSchema.Fields[k]
			// Bypass SDT if not float, or if the field is in mini-cold start (StateLearning) or schema drift (StateFlagged)
			if !ok || f.Type != "float" || f.State == schema.StateLearning || f.State == schema.StateFlagged {
				fieldsToKeep[k] = v
				shouldEmit = true
				continue
			}

			if v == nil {
				continue
			}

			floatVal, isFloat := v.(float64)
			if !isFloat {
				continue
			}

			if heartbeatTriggered {
				f.LastTime = payloadTime
				f.LastValue = floatVal
				fieldsToKeep[k] = f.LastValue
				
				f.AnchorTime = payloadTime
				f.AnchorValue = floatVal
				f.MaxLowerSlope = math.Inf(-1)
				f.MinUpperSlope = math.Inf(1)
				shouldEmit = true
				
				saveSDT(p, sensorID, k, f)
				continue
			}

			if !f.SDTRestored {
				restoreSDT(p, sensorID, k, f)
			}

			// Bootstrap Step
			if f.AnchorTime.IsZero() {
				shouldEmitField, _, _, _ := EvaluateFieldSDT(f, floatVal, payloadTime)
				fieldsToKeep[k] = floatVal
				if shouldEmitField {
					shouldEmit = true
					saveSDT(p, sensorID, k, f)
				}
				continue
			}

			shouldEmitField, emitValue, emitTime, archived := EvaluateFieldSDT(f, floatVal, payloadTime)
			if shouldEmitField {
				fieldsToKeep[k] = emitValue
				shouldEmit = true
				if archived {
					if !useArchivedTime || emitTime.After(archivedTime) {
						useArchivedTime = true
						archivedTime = emitTime
					}
				}
				saveSDT(p, sensorID, k, f)
			}
		}

		if shouldEmit {
			sensorSchema.LastHeartbeatTime = time.Now()
		}
	}

	if !shouldEmit {
		// Suppress completely
		return nil, nil
	}

	// 3. Augment payload
	data := map[string]interface{}{
		"sensor_id":         sensorID,
		"timestamp":         payloadTime.UTC().Format(time.RFC3339Nano),
		"fields":            fieldsToKeep,
		"tier":              tier,
		"cold_start_active": coldStartActive,
	}

	if useArchivedTime {
		data["timestamp"] = archivedTime.UTC().Format(time.RFC3339Nano)
	}

	if len(inferResult.Flags) > 0 {
		data["flags"] = inferResult.Flags
	} else {
		data["flags"] = nil
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output json: %w", err)
	}

	return out, nil
}

func restoreSDT(proc *Processor, sensorID, fieldName string, f *schema.Field) {
	if proc.Resil == nil {
		f.SDTRestored = true
		return
	}
	rdb := proc.Resil.GetRedis()
	if rdb == nil {
		f.SDTRestored = true
		return
	}
	
	key := fmt.Sprintf("sdt_state:%s:%s", sensorID, fieldName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	vals, err := rdb.HGetAll(ctx, key).Result()
	if err != nil || len(vals) < 6 {
		// Missing, corrupt, or partial write. Treat as not found (bootstrap).
		f.SDTRestored = true
		return
	}
	
	if val, ok := vals["AnchorValue"]; ok {
		f.AnchorValue, _ = strconv.ParseFloat(val, 64)
	} else { return }
	if val, ok := vals["AnchorTime"]; ok {
		f.AnchorTime, _ = time.Parse(time.RFC3339Nano, val)
	} else { return }
	if val, ok := vals["LastValue"]; ok {
		f.LastValue, _ = strconv.ParseFloat(val, 64)
	} else { return }
	if val, ok := vals["LastTime"]; ok {
		f.LastTime, _ = time.Parse(time.RFC3339Nano, val)
	} else { return }
	if val, ok := vals["MaxLowerSlope"]; ok {
		f.MaxLowerSlope, _ = strconv.ParseFloat(val, 64)
	} else { return }
	if val, ok := vals["MinUpperSlope"]; ok {
		f.MinUpperSlope, _ = strconv.ParseFloat(val, 64)
	} else { return }
	
	f.SDTRestored = true
}

func saveSDT(proc *Processor, sensorID, fieldName string, f *schema.Field) {
	if proc.Resil == nil {
		return
	}
	rdb := proc.Resil.GetRedis()
	if rdb == nil {
		return
	}
	
	key := fmt.Sprintf("sdt_state:%s:%s", sensorID, fieldName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Write atomically using single HSET call
	rdb.HSet(ctx, key,
		"AnchorValue", strconv.FormatFloat(f.AnchorValue, 'f', -1, 64),
		"AnchorTime", f.AnchorTime.Format(time.RFC3339Nano),
		"LastValue", strconv.FormatFloat(f.LastValue, 'f', -1, 64),
		"LastTime", f.LastTime.Format(time.RFC3339Nano),
		"MaxLowerSlope", strconv.FormatFloat(f.MaxLowerSlope, 'f', -1, 64),
		"MinUpperSlope", strconv.FormatFloat(f.MinUpperSlope, 'f', -1, 64),
	)
}

// CheckHeartbeats scans the registry for any sensors that haven't sent a message recently,
// indicating a missing heartbeat / sensor dropout, and returns synthetic flag payloads.
func (p *Processor) CheckHeartbeats() [][]byte {
	var outputs [][]byte
	
	sensors := p.Registry.GetAllSensors()
	now := time.Now()
	
	for _, s := range sensors {
		if s.HeartbeatInterval == 0 {
			continue
		}
		
		if !s.LastSeenTime.IsZero() && now.Sub(s.LastSeenTime) > s.HeartbeatInterval {
			if !s.DropoutFlagged {
				s.DropoutFlagged = true
				
				// Construct synthetic dropout payload
				data := map[string]interface{}{
					"sensor_id": s.ID,
					"timestamp": now.UTC().Format(time.RFC3339Nano),
					"tier":      s.Tier,
					"fields":    make(map[string]interface{}),
					"flags": map[string]string{
						"sensor": "sensor_dropout: no heartbeat received within interval",
					},
					"cold_start_active": false,
				}
				
				out, _ := json.Marshal(data)
				outputs = append(outputs, out)
			}
		}
	}
	
	return outputs
}

// EvaluateFieldSDT computes the core Swinging Door Trending mathematical logic.
// It modifies the provided schema.Field state in-place.
// Returns:
// shouldEmit: true if a corridor break (or bootstrap) occurred and a point must be emitted.
// emitValue: the value to emit (N-1 for breaks).
// emitTime: the time of the emitted value.
// useArchivedTime: true if the emitted value comes from an earlier timestamp than payloadTime.
func EvaluateFieldSDT(f *schema.Field, floatVal float64, payloadTime time.Time) (shouldEmit bool, emitValue float64, emitTime time.Time, useArchivedTime bool) {
	// Bootstrap Step
	if f.AnchorTime.IsZero() {
		f.AnchorTime = payloadTime
		f.AnchorValue = floatVal
		f.LastTime = payloadTime
		f.LastValue = floatVal
		f.MaxLowerSlope = math.Inf(-1)
		f.MinUpperSlope = math.Inf(1)
		return true, floatVal, payloadTime, false
	}

	dt := payloadTime.Sub(f.AnchorTime).Seconds()
	if dt <= 0 {
		// Zero or negative dt - fallback to tiny dt to avoid division by zero or skip
		// but the test expects to skip this point for <= 0 dt!
		// Let's implement skip for dt <= 0 based on test expectations
		return false, 0, time.Time{}, false
	}

	upperSlope := (floatVal + f.DoorWidth - f.AnchorValue) / dt
	lowerSlope := (floatVal - f.DoorWidth - f.AnchorValue) / dt

	minUpper := math.Min(f.MinUpperSlope, upperSlope)
	maxLower := math.Max(f.MaxLowerSlope, lowerSlope)

	if maxLower > minUpper {
		// Corridor Broken -> Emit N-1 (LastValue)
		emitValue = f.LastValue
		emitTime = f.LastTime
		shouldEmit = true
		useArchivedTime = true

		// Re-Anchor on N-1
		f.AnchorTime = f.LastTime
		f.AnchorValue = f.LastValue

		// Update last seen
		f.LastValue = floatVal
		f.LastTime = payloadTime

		// Re-Evaluate P_new against new corridor immediately
		dt2 := payloadTime.Sub(f.AnchorTime).Seconds()
		if dt2 <= 0 {
			dt2 = 0.001
		}
		f.MinUpperSlope = (floatVal + f.DoorWidth - f.AnchorValue) / dt2
		f.MaxLowerSlope = (floatVal - f.DoorWidth - f.AnchorValue) / dt2

		return true, emitValue, emitTime, true
	} else {
		// Point inside corridor
		f.MinUpperSlope = minUpper
		f.MaxLowerSlope = maxLower
		f.LastTime = payloadTime
		f.LastValue = floatVal
		return false, 0, time.Time{}, false
	}
}
