package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"zaee-engine/pkg/config"
	"zaee-engine/pkg/schema"
)

type FusedPayload struct {
	SensorID  string
	Timestamp time.Time
	Fields    map[string]interface{}
	Flags     map[string]string
}

type bucketKey struct {
	sensorID string
	bucket   int64
}

type bucketState struct {
	timestamp time.Time
	fields    map[string]interface{}
	flags     map[string]string
	expiresAt time.Time
}

type Manager struct {
	rdb      *redis.Client
	registry *schema.Registry
	cfg      *config.Config

	mu      sync.Mutex
	buffers map[bucketKey]*bucketState
	outChan chan FusedPayload
}

func NewManager(rdb *redis.Client, reg *schema.Registry, cfg *config.Config) (*Manager, error) {

	m := &Manager{
		rdb:      rdb,
		registry: reg,
		cfg:      cfg,
		buffers:  make(map[bucketKey]*bucketState),
		outChan:  make(chan FusedPayload, 100),
	}
	go m.tickerLoop()
	return m, nil
}

func (m *Manager) OutChan() <-chan FusedPayload {
	return m.outChan
}

func (m *Manager) AddMessage(sensorID string, timestamp time.Time, fields map[string]interface{}, inferResult schema.InferResult) {
	sSchema, exists := m.registry.GetSensor(sensorID)
	if !exists {
		return
	}

	windowDuration := sSchema.WindowDuration
	if windowDuration <= 0 {
		var err error
		windowDuration, err = time.ParseDuration(m.cfg.Engine.DefaultWindow)
		if err != nil || windowDuration <= 0 {
			windowDuration = 500 * time.Millisecond
		}
	}

	bucket := timestamp.Truncate(windowDuration)
	key := bucketKey{sensorID: sensorID, bucket: bucket.UnixNano()}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this is a late arrival (bucket has already expired according to grace period)
	gracePeriod := windowDuration + 50*time.Millisecond
	if time.Since(bucket) > gracePeriod {
		// Late arrival -> go straight to Redis state, do not buffer
		ctx := context.Background()
		ttl := sSchema.HeartbeatInterval * 2
		if ttl <= 0 {
			ttl = 2 * time.Minute
		}
		if m.rdb != nil {
			for k, v := range fields {
				redisKey := fmt.Sprintf("state:%s:%s", sensorID, k)
				b, _ := json.Marshal(v)
				m.rdb.Set(ctx, redisKey, b, ttl)
			}
		}
		return
	}

	state, ok := m.buffers[key]
	if !ok {
		state = &bucketState{
			timestamp: timestamp, // Keep first timestamp or bucket timestamp? Keep the first real timestamp for accuracy
			fields:    make(map[string]interface{}),
			flags:     make(map[string]string),
			expiresAt: time.Now().Add(gracePeriod),
		}
		m.buffers[key] = state
	} else {
		// Update timestamp to max timestamp in bucket if needed, or keep first. Keeping first is fine for LOCF anchor.
		if timestamp.After(state.timestamp) {
			state.timestamp = timestamp
		}
	}

	for k, v := range fields {
		state.fields[k] = v
	}
	for k, v := range inferResult.Flags {
		state.flags[k] = v
	}
}

func (m *Manager) tickerLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		<-ticker.C
		m.flushExpired()
	}
}

func (m *Manager) flushExpired() {
	m.mu.Lock()
	var expiredKeys []bucketKey
	now := time.Now()
	for k, state := range m.buffers {
		if now.After(state.expiresAt) {
			expiredKeys = append(expiredKeys, k)
		}
	}

	// We pop them off under the lock
	expiredBuckets := make(map[bucketKey]*bucketState)
	for _, k := range expiredKeys {
		expiredBuckets[k] = m.buffers[k]
		delete(m.buffers, k)
	}
	m.mu.Unlock()

	// Process them outside the lock to avoid holding lock during Redis queries
	for key, state := range expiredBuckets {
		m.processBucket(key, state)
	}
}

func (m *Manager) processBucket(key bucketKey, state *bucketState) {
	sSchema, exists := m.registry.GetSensor(key.sensorID)
	if !exists {
		return
	}

	ctx := context.Background()
	flags := make(map[string]string)
	
	// Pre-populate with flags from schema inference
	for k, v := range state.flags {
		flags[k] = v
	}

	ttl := sSchema.HeartbeatInterval * 2
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}

	if m.rdb != nil {
		for k, v := range state.fields {
			redisKey := fmt.Sprintf("state:%s:%s", key.sensorID, k)
			b, _ := json.Marshal(v)
			m.rdb.Set(ctx, redisKey, b, ttl)
		}
	}

	// 2. For fields that did NOT arrive, LOCF from Redis
	// We need to know ALL fields this sensor is supposed to have.
	// That's what schema.Fields gives us.
	for expectedField := range sSchema.Fields {
		if _, ok := state.fields[expectedField]; !ok {
			if m.rdb == nil {
				flags[expectedField] = "field_unavailable"
				continue
			}
			// Missing! Fetch from Redis
			redisKey := fmt.Sprintf("state:%s:%s", key.sensorID, expectedField)
			valStr, err := m.rdb.Get(ctx, redisKey).Result()
			if err == redis.Nil || err != nil {
				// TTL expired or never seen
				flags[expectedField] = "field_unavailable"
			} else {
				// LOCF successful
				var val interface{}
				if err := json.Unmarshal([]byte(valStr), &val); err == nil {
					state.fields[expectedField] = val
					flags[expectedField] = "locf_gap_filled"
				} else {
					flags[expectedField] = "field_unavailable"
				}
			}
		}
	}

	m.outChan <- FusedPayload{
		SensorID:  key.sensorID,
		Timestamp: state.timestamp,
		Fields:    state.fields,
		Flags:     flags,
	}
}
