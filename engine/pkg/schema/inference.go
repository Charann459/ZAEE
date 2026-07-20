package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"zaee-engine/pkg/resilience"
)

// Infer inspects incoming raw fields and updates the registry.
// Returns a list of flag messages if there are any schema violations, and a boolean indicating if cold start is active.
func (r *Registry) Infer(sensorID string, rawFields map[string]interface{}, timestamp time.Time, resil *resilience.Resilience) (InferResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := InferResult{
		Flags:           make(map[string]string),
		DriftPromotions: make([]string, 0),
	}

	schema, exists := r.sensors[sensorID]
	if !exists {
		// First time seeing this sensor, build initial schema
		schema = &SensorSchema{
			ID:     sensorID,
			Tier:   "standard", // default unless overridden by config
			Fields: make(map[string]*Field),
			Checklist: Checklist{
				Active:          true,
				TotalSamples:    0,
				RequiredSamples: 50,
			},
			HeartbeatInterval: 60 * time.Second, // Default 60s
		}
		r.sensors[sensorID] = schema
		
		// Attempt to load checkpoints
		if resil != nil {
			metadata, err := resil.LoadCheckpointsSafe(sensorID)
			if err == nil && len(metadata) > 0 {
				// Reconstruct schema state
				var state struct {
					Checklist      Checklist           `json:"checklist"`
					WindowDuration time.Duration       `json:"window_duration"`
					Fields         map[string]*Field   `json:"fields"`
				}
				if json.Unmarshal(metadata, &state) == nil {
					schema.Checklist = state.Checklist
					schema.WindowDuration = state.WindowDuration
					schema.Fields = state.Fields
					fmt.Printf("[Schema Registry] Restored Cold Start state for %s from PostgreSQL (TotalSamples: %d)\n", sensorID, schema.Checklist.TotalSamples)
				} else {
					fmt.Printf("[Schema Registry] Failed to parse checkpoint for %s, entering Cold Start\n", sensorID)
				}
			} else {
				fmt.Printf("[Schema Registry] Discovered new sensor: %s, entering Cold Start\n", sensorID)
			}
		} else {
			fmt.Printf("[Schema Registry] Discovered new sensor: %s, entering Cold Start\n", sensorID)
		}
	} else if schema.Checklist.RequiredSamples == 0 {
		// For pre-declared sensors without a checklist initialized
		schema.Checklist = Checklist{
			Active:          schema.Tier != "critical", // Critical tier bypasses cold start immediately
			TotalSamples:    0,
			RequiredSamples: 50,
		}
		if schema.HeartbeatInterval == 0 {
			schema.HeartbeatInterval = 60 * time.Second
		}
		
		if schema.Tier == "critical" {
			fmt.Printf("[Schema Registry] Sensor '%s' is Critical Tier. Bypassing Cold Start.\n", sensorID)
		}
	}
	
	// Increment sensor level total samples
	if schema.Checklist.Active {
		schema.Checklist.TotalSamples++
		if schema.Checklist.TotalSamples == 1 {
			schema.Checklist.StartTime = timestamp
		}
	}

	// Check fields
	for k, v := range rawFields {
		detectedType := InferFieldType(v)
		isFloat := (detectedType == "float")
		var floatVal float64
		if isFloat {
			floatVal = v.(float64)
		}

		field, ok := schema.Fields[k]
		if !ok {
			// New field discovered
			newField := &Field{
				Name:  k,
				Type:  detectedType,
			}
			
			if schema.Checklist.Active {
				newField.State = StateLearned
			} else {
				newField.State = StateLearning
				newField.Checklist = Checklist{
					Active:          true,
					TotalSamples:    0,
					RequiredSamples: 25, // Individual field default
					StartTime:       timestamp,
				}
				result.Flags[k] = "learning_new_field"
			}
			
			// Initialize baseline if it's a float
			if isFloat {
				newField.Baseline = Baseline{
					Count: 1,
					Sum:   floatVal,
					SumSq: floatVal * floatVal,
					Min:   floatVal,
					Max:   floatVal,
					Mean:  floatVal,
				}
			}

			schema.Fields[k] = newField
			fmt.Printf("[Schema Registry] Sensor '%s' discovered new field '%s' (type: %s)\n", sensorID, k, detectedType)
			
		} else {
			// Existing field
			if field.Type == "" {
				field.Type = detectedType
				fmt.Printf("[Schema Registry] Sensor '%s' learned pre-declared field '%s' is type: %s\n", sensorID, k, detectedType)
			} else if field.Type != detectedType {
				// Type Mismatch (Schema Drift)
				if field.State != StateFlagged || field.DriftType == "" {
					// First mismatch
					field.State = StateFlagged
					field.DriftType = detectedType
					field.Checklist = Checklist{
						Active:          true,
						TotalSamples:    1,
						RequiredSamples: 25,
						StartTime:       timestamp,
					}
					if isFloat {
						field.Baseline = Baseline{Count: 1, Sum: floatVal, SumSq: floatVal * floatVal, Min: floatVal, Max: floatVal, Mean: floatVal}
					}
					result.Flags[k] = fmt.Sprintf("schema_drift: expected %s, received %s", field.Type, detectedType)
				} else if field.DriftType != detectedType {
					// Oscillating Drift
					field.DriftType = detectedType
					field.Checklist = Checklist{
						Active:          true,
						TotalSamples:    1,
						RequiredSamples: 25,
						StartTime:       timestamp,
					}
					if isFloat {
						field.Baseline = Baseline{Count: 1, Sum: floatVal, SumSq: floatVal * floatVal, Min: floatVal, Max: floatVal, Mean: floatVal}
					}
					result.Flags[k] = fmt.Sprintf("schema_drift: oscillating to %s", detectedType)
				} else {
					// Continuing matched drift
					field.Checklist.TotalSamples++
					if isFloat {
						UpdateBaseline(&field.Baseline, floatVal)
						if floatVal < field.Baseline.Min { field.Baseline.Min = floatVal }
						if floatVal > field.Baseline.Max { field.Baseline.Max = floatVal }
					}
					result.Flags[k] = fmt.Sprintf("schema_drift: expected %s, received %s", field.Type, detectedType)
				}
			} else {
				// Matches original type
				if field.State == StateFlagged && field.DriftType != "" {
					// Abort drift
					field.State = StateLearned
					field.DriftType = ""
					field.Checklist.Active = false
				}
				
				if field.State == StateLearning {
					// Mini cold start progression
					field.Checklist.TotalSamples++
					if isFloat {
						UpdateBaseline(&field.Baseline, floatVal)
						if floatVal < field.Baseline.Min { field.Baseline.Min = floatVal }
						if floatVal > field.Baseline.Max { field.Baseline.Max = floatVal }
					}
					result.Flags[k] = "learning_new_field"
				} else if isFloat && schema.Checklist.Active {
					// Main cold start progression
					UpdateBaseline(&field.Baseline, floatVal)
					if field.Baseline.Count == 1 {
						field.Baseline.Min = floatVal
						field.Baseline.Max = floatVal
					} else {
						if floatVal < field.Baseline.Min { field.Baseline.Min = floatVal }
						if floatVal > field.Baseline.Max { field.Baseline.Max = floatVal }
					}
				}
			}
		}

		// Check for individual field checklist completion (Drift or Learning)
		field = schema.Fields[k] // Refresh pointer reference just in case
		if field.Checklist.Active && field.Checklist.TotalSamples >= field.Checklist.RequiredSamples {
			field.Checklist.Active = false
			
			if field.State == StateFlagged {
				field.Type = field.DriftType
				field.DriftType = ""
				field.State = StateLearned
				if field.Type == "float" {
					field.DoorWidth = CalculateDoorWidth(field.Baseline.Mean, field.Baseline.Variance)
				}
				result.DriftPromotions = append(result.DriftPromotions, k)
				fmt.Printf("[Schema Registry] Sensor '%s' field '%s' drift promoted to type %s\n", sensorID, k, field.Type)
			} else if field.State == StateLearning {
				field.State = StateLearned
				if field.Type == "float" {
					field.DoorWidth = CalculateDoorWidth(field.Baseline.Mean, field.Baseline.Variance)
				}
				result.DriftPromotions = append(result.DriftPromotions, k)
				fmt.Printf("[Schema Registry] Sensor '%s' field '%s' completed mini-cold start.\n", sensorID, k)
			}
			
			delete(result.Flags, k)

			if resil != nil {
				metadata := map[string]interface{}{
					"checklist":       schema.Checklist,
					"window_duration": schema.WindowDuration,
					"fields":          schema.Fields,
				}
				resil.WriteCheckpointSafe(sensorID, "field_promotion_"+k, metadata)
			}
		}
	}
	
	// Check if main cold start is complete
	if schema.Checklist.Active && schema.Checklist.TotalSamples >= schema.Checklist.RequiredSamples {
		schema.Checklist.Active = false
		fmt.Printf("[Schema Registry] Sensor '%s' completed Cold Start. Baselines established.\n", sensorID)
		
		// Calculate WindowDuration
		elapsed := timestamp.Sub(schema.Checklist.StartTime)
		if elapsed > 0 && schema.Checklist.TotalSamples > 0 {
			avgInterval := elapsed / time.Duration(schema.Checklist.TotalSamples)
			schema.WindowDuration = 2 * avgInterval
		}
		if schema.WindowDuration <= 0 {
			schema.WindowDuration = 500 * time.Millisecond // fallback
		}
		fmt.Printf("[Schema Registry] Derived WindowDuration: %v for %s\n", schema.WindowDuration, sensorID)

		// Initialize DoorWidths for SDT
		for _, f := range schema.Fields {
			if f.Type == "float" {
				f.DoorWidth = CalculateDoorWidth(f.Baseline.Mean, f.Baseline.Variance)
				fmt.Printf("  - Field '%s': Mean=%.2f, Var=%.2f -> DoorWidth=%.4f\n", f.Name, f.Baseline.Mean, f.Baseline.Variance, f.DoorWidth)
			}
		}

		if resil != nil {
			// Construct metadata to persist
			metadata := map[string]interface{}{
				"checklist":       schema.Checklist,
				"window_duration": schema.WindowDuration,
				"fields":          schema.Fields,
			}
			resil.WriteCheckpointSafe(sensorID, "cold_start_completed", metadata)
		}
	} else if schema.Checklist.Active && schema.Checklist.TotalSamples > 0 && schema.Checklist.TotalSamples%25 == 0 {
		// Milestone checkpoint every 25 samples
		if resil != nil {
			metadata := map[string]interface{}{
				"checklist": schema.Checklist,
				"fields":    schema.Fields,
			}
			resil.WriteCheckpointSafe(sensorID, fmt.Sprintf("sample_collection_%d", schema.Checklist.TotalSamples), metadata)
		}
	}

	return result, schema.Checklist.Active
}

func InferFieldType(v interface{}) string {
	switch v.(type) {
	case float64:
		return "float"
	case string:
		return "string"
	case bool:
		return "bool"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func UpdateBaseline(b *Baseline, floatVal float64) {
	b.Count++
	countF := float64(b.Count)
	delta := floatVal - b.Mean
	b.Sum += floatVal
	b.SumSq += floatVal * floatVal
	b.Mean += delta / countF
	b.Variance = (b.SumSq / countF) - (b.Mean * b.Mean)
}

func CalculateDoorWidth(mean, variance float64) float64 {
	door := 2 * math.Sqrt(variance)
	floor := math.Abs(0.01 * mean)
	if door < floor {
		door = floor
	}
	if door < 0.001 {
		door = 0.001
	}
	return door
}
