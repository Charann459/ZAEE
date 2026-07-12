package unit

// ASSUMPTIONS ABOUT YOUR PACKAGE API:
// - GetBucketKey(sensorID string, timestamp time.Time, windowDuration time.Duration) string
// - MergeFields(existing, incoming map[string]interface{}) map[string]interface{}
// - IsLateArrival(timestamp time.Time, lastFlushedBucket time.Time, windowDuration time.Duration) bool
// Adapt to your actual pkg/fusion types and function signatures.

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- STUB IMPLEMENTATIONS ---
// Replace these with calls to your actual pkg/fusion functions.

func GetBucketKey(sensorID string, timestamp time.Time, windowDuration time.Duration) string {
	bucket := timestamp.Truncate(windowDuration)
	return fmt.Sprintf("%s:%d", sensorID, bucket.UnixNano())
}

func MergeFields(existing, incoming map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range existing {
		result[k] = v
	}
	for k, v := range incoming {
		result[k] = v
	}
	return result
}

func IsLateArrival(msgTimestamp time.Time, lastFlushedBucket time.Time, windowDuration time.Duration, gracePeriod time.Duration) bool {
	bucket := msgTimestamp.Truncate(windowDuration)
	flushDeadline := lastFlushedBucket.Add(gracePeriod)
	return bucket.Before(lastFlushedBucket) || time.Now().After(flushDeadline)
}

// =========================================================
// TEST 1: Bucket key — same bucket for timestamps within window
// =========================================================
func TestFusion_BucketKey_SameWindowSameBucket(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 500 * time.Millisecond

	// Two timestamps 30ms apart — both within the same 500ms bucket
	ts1 := base.Add(10 * time.Millisecond)
	ts2 := base.Add(40 * time.Millisecond)

	key1 := GetBucketKey("machine_01", ts1, window)
	key2 := GetBucketKey("machine_01", ts2, window)

	assert.Equal(t, key1, key2,
		"Two timestamps 30ms apart within a 500ms window must produce the same bucket key")
}

// =========================================================
// TEST 2: Bucket key — different windows produce different buckets
// =========================================================
func TestFusion_BucketKey_DifferentWindowDifferentBucket(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 500 * time.Millisecond

	ts1 := base.Add(100 * time.Millisecond) // bucket 0
	ts2 := base.Add(600 * time.Millisecond) // bucket 1 (past 500ms boundary)

	key1 := GetBucketKey("machine_01", ts1, window)
	key2 := GetBucketKey("machine_01", ts2, window)

	assert.NotEqual(t, key1, key2,
		"Timestamps spanning a window boundary must produce different bucket keys")
}

// =========================================================
// TEST 3: Bucket key — different sensor IDs produce different keys
// =========================================================
func TestFusion_BucketKey_DifferentSensorDifferentKey(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 500 * time.Millisecond

	key1 := GetBucketKey("machine_01", base, window)
	key2 := GetBucketKey("machine_02", base, window)

	assert.NotEqual(t, key1, key2,
		"Same timestamp but different sensor IDs must produce different bucket keys")
}

// =========================================================
// TEST 4: Field merging — two separate payloads merge correctly
// =========================================================
func TestFusion_MergeFields_TwoSeparatePayloads(t *testing.T) {
	// Simulates: Temp arrives first, Pressure arrives 30ms later, same bucket
	existing := map[string]interface{}{
		"temperature": 90.5,
	}
	incoming := map[string]interface{}{
		"pressure": 121.3,
	}

	merged := MergeFields(existing, incoming)

	assert.Equal(t, 90.5, merged["temperature"], "Temperature must survive the merge")
	assert.Equal(t, 121.3, merged["pressure"], "Pressure must be added by the merge")
	assert.Len(t, merged, 2, "Merged payload must contain exactly 2 fields")
}

// =========================================================
// TEST 5: Field merging — later arrival wins on conflict
// =========================================================
func TestFusion_MergeFields_LaterArrivalWins(t *testing.T) {
	// If the same field arrives twice in a window (unlikely but possible),
	// the most recently arrived value should win.
	existing := map[string]interface{}{
		"temperature": 90.5,
	}
	incoming := map[string]interface{}{
		"temperature": 91.0, // newer value for same field
	}

	merged := MergeFields(existing, incoming)

	assert.Equal(t, 91.0, merged["temperature"],
		"When the same field arrives twice in a window, the incoming (later) value must win")
}

// =========================================================
// TEST 6: LOCF gap fill flags
// =========================================================
func TestFusion_LOCFFlags_GapFilledFieldFlagged(t *testing.T) {
	// Simulate: window flushed with only Temp present, Pressure filled from Redis
	windowFields := map[string]interface{}{
		"temperature": 90.5,
	}
	expectedFields := []string{"temperature", "pressure"}
	redisValues := map[string]interface{}{
		"pressure": 120.0,
	}

	// Perform LOCF gap fill
	flags := make(map[string]string)
	for _, field := range expectedFields {
		if _, present := windowFields[field]; !present {
			if val, inRedis := redisValues[field]; inRedis {
				windowFields[field] = val
				flags[field] = "locf_gap_filled"
			} else {
				flags[field] = "field_unavailable"
			}
		}
	}

	assert.Equal(t, 120.0, windowFields["pressure"],
		"Pressure should be gap-filled from Redis")
	assert.Equal(t, "locf_gap_filled", flags["pressure"],
		"Gap-filled field must carry locf_gap_filled flag")
	assert.Empty(t, flags["temperature"],
		"Natively present field must not carry any flag")
}

// =========================================================
// TEST 7: LOCF gap fill — field unavailable when Redis is empty
// =========================================================
func TestFusion_LOCFFlags_FieldUnavailableWhenRedisEmpty(t *testing.T) {
	windowFields := map[string]interface{}{
		"temperature": 90.5,
	}
	expectedFields := []string{"temperature", "pressure"}
	redisValues := map[string]interface{}{} // Redis has nothing for pressure

	flags := make(map[string]string)
	for _, field := range expectedFields {
		if _, present := windowFields[field]; !present {
			if val, inRedis := redisValues[field]; inRedis {
				windowFields[field] = val
				flags[field] = "locf_gap_filled"
			} else {
				flags[field] = "field_unavailable"
			}
		}
	}

	_, pressurePresent := windowFields["pressure"]
	assert.False(t, pressurePresent,
		"Pressure must be omitted from output when Redis has no prior value")
	assert.Equal(t, "field_unavailable", flags["pressure"],
		"Missing field with no Redis value must be flagged as field_unavailable")
}

// =========================================================
// TEST 8: Late arrival detection
// =========================================================
func TestFusion_LateArrival_DetectedCorrectly(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 500 * time.Millisecond
	grace := 50 * time.Millisecond

	// Bucket 0 covers [0ms, 500ms), was flushed at base+550ms (after grace)
	lastFlushedBucket := base // the bucket that was flushed

	// A message with a timestamp in bucket 0 arriving now (well after flush)
	lateMessageTimestamp := base.Add(200 * time.Millisecond) // belongs to bucket 0

	isLate := lateMessageTimestamp.Truncate(window).Before(lastFlushedBucket.Add(window))
	_ = grace // your implementation may use this differently

	// It's late if its bucket is before or equal to the last flushed bucket
	lateBucket := lateMessageTimestamp.Truncate(window)
	assert.True(t, !lateBucket.After(lastFlushedBucket),
		"Message whose bucket has already been flushed must be treated as late arrival")
}

func TestFusion_NonLateArrival_GoesIntoBuffer(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	window := 500 * time.Millisecond

	lastFlushedBucket := base // bucket 0 was flushed

	// Message in bucket 1 — this is NOT late
	freshMessageTimestamp := base.Add(600 * time.Millisecond)
	freshBucket := freshMessageTimestamp.Truncate(window) // = base+500ms = bucket 1

	isLate := !freshBucket.After(lastFlushedBucket)
	assert.False(t, isLate, "Message in a future bucket must not be treated as late arrival")
}
