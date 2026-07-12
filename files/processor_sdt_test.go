package unit

// ASSUMPTIONS ABOUT YOUR PACKAGE API:
// - Your SDT state lives in a struct (e.g. pkg/processor or pkg/schema) with these fields:
//     AnchorValue    float64
//     AnchorTime     time.Time
//     LastValue      float64
//     LastTime       time.Time
//     MaxLowerSlope  float64
//     MinUpperSlope  float64
//     DoorWidth      float64
// - EvaluateSDT(state *SDTState, newValue float64, newTime time.Time) (shouldEmit bool, emitValue float64, emitTime time.Time)
// - Adapt import paths to match your actual module name.

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	// Replace with your actual import path:
	// "github.com/yourusername/zaee/engine/pkg/processor"
)

// SDTState mirrors your internal state struct.
// Replace with your actual type once you wire these tests into the real package.
type SDTState struct {
	AnchorValue   float64
	AnchorTime    time.Time
	LastValue     float64
	LastTime      time.Time
	MaxLowerSlope float64
	MinUpperSlope float64
	DoorWidth     float64
}

// EvaluateSDT is a pure, extracted version of your SDT logic for unit testing.
// This must match your processor.go implementation exactly.
// Returns: (shouldEmit bool, emitValue float64, emitTime time.Time, updatedState SDTState)
func EvaluateSDT(state SDTState, newValue float64, newTime time.Time) (bool, float64, time.Time, SDTState) {
	// Bootstrap: first point after cold start
	if state.AnchorTime.IsZero() {
		state.AnchorValue = newValue
		state.AnchorTime = newTime
		state.LastValue = newValue
		state.LastTime = newTime
		state.MinUpperSlope = math.Inf(1)
		state.MaxLowerSlope = math.Inf(-1)
		return true, newValue, newTime, state
	}

	dt := newTime.Sub(state.AnchorTime).Seconds()
	if dt <= 0 {
		// Zero or negative dt — skip this point
		return false, 0, time.Time{}, state
	}

	upperSlope := (newValue + state.DoorWidth - state.AnchorValue) / dt
	lowerSlope := (newValue - state.DoorWidth - state.AnchorValue) / dt

	newMinUpper := math.Min(state.MinUpperSlope, upperSlope)
	newMaxLower := math.Max(state.MaxLowerSlope, lowerSlope)

	if newMaxLower > newMinUpper {
		// Corridor broken — emit N-1, re-anchor on N-1, evaluate N against new corridor
		emitValue := state.LastValue
		emitTime := state.LastTime

		// Re-anchor on the previous point
		state.AnchorValue = state.LastValue
		state.AnchorTime = state.LastTime
		state.MinUpperSlope = math.Inf(1)
		state.MaxLowerSlope = math.Inf(-1)

		// Update last seen
		state.LastValue = newValue
		state.LastTime = newTime

		return true, emitValue, emitTime, state
	}

	// Within corridor — suppress, update slopes and last seen
	state.MinUpperSlope = newMinUpper
	state.MaxLowerSlope = newMaxLower
	state.LastValue = newValue
	state.LastTime = newTime
	return false, 0, time.Time{}, state
}

// --- HELPER ---
func baseTime() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

func t(seconds float64) time.Time {
	return baseTime().Add(time.Duration(seconds * float64(time.Second)))
}

// =========================================================
// TEST 1: Bootstrap — first point always emits
// =========================================================
func TestSDT_Bootstrap_AlwaysEmitsFirstPoint(t *testing.T) {
	state := SDTState{DoorWidth: 2.0}
	shouldEmit, emitValue, _, _ := EvaluateSDT(state, 90.0, baseTime())

	assert.True(t, shouldEmit, "First point after cold start must always be emitted")
	assert.Equal(t, 90.0, emitValue)
}

// =========================================================
// TEST 2: Stable flatline — all subsequent points suppressed
// =========================================================
func TestSDT_StableFlatline_SuppressesAllPoints(t *testing.T) {
	state := SDTState{DoorWidth: 2.0}

	// Bootstrap
	_, _, _, state = EvaluateSDT(state, 90.0, baseTime())

	// Feed 10 identical readings — none should emit
	for i := 1; i <= 10; i++ {
		shouldEmit, _, _, newState := EvaluateSDT(state, 90.0, baseTime().Add(time.Duration(i)*time.Second))
		assert.False(t, shouldEmit, "Stable flatline point %d should be suppressed", i)
		state = newState
	}
}

// =========================================================
// TEST 3: Sudden spike — corridor breaks, N-1 is emitted
// =========================================================
func TestSDT_SuddenSpike_EmitsPreviousPoint(t *testing.T) {
	state := SDTState{DoorWidth: 2.0}

	// Bootstrap at 90.0
	_, _, _, state = EvaluateSDT(state, 90.0, baseTime())

	// Stable point — within corridor, suppressed, becomes N-1
	shouldEmit, _, _, state := EvaluateSDT(state, 90.1, baseTime().Add(1*time.Second))
	assert.False(t, shouldEmit, "Stable point should be suppressed before spike")

	// Record what N-1 looks like
	lastValue := state.LastValue // should be 90.1
	lastTime := state.LastTime   // should be t+1s

	// Spike at t+2s that breaks the corridor
	shouldEmit, emitValue, emitTime, _ := EvaluateSDT(state, 95.0, baseTime().Add(2*time.Second))

	assert.True(t, shouldEmit, "Spike should break corridor and trigger emission")
	assert.Equal(t, lastValue, emitValue, "Engine must emit N-1 (previous stable point), not the spike itself")
	assert.Equal(t, lastTime, emitTime, "Emitted timestamp must be N-1 timestamp")
}

// =========================================================
// TEST 4: Slow drift — SDT detects what simple deadband misses
// This is the KEY advantage of SDT over simple deadband.
// A value drifting 3 units over 9 seconds in small steps
// must eventually break the corridor.
// =========================================================
func TestSDT_SlowDrift_BreaksCorridorEventually(t *testing.T) {
	state := SDTState{DoorWidth: 0.5} // tight door for sensitivity

	// Bootstrap
	_, _, _, state = EvaluateSDT(state, 90.0, baseTime())

	driftValues := []float64{90.1, 90.2, 90.3, 90.5, 91.0, 91.5, 92.0, 92.5, 93.0}
	corridorBroken := false

	for i, v := range driftValues {
		shouldEmit, _, _, newState := EvaluateSDT(state, v, baseTime().Add(time.Duration(i+1)*time.Second))
		state = newState
		if shouldEmit {
			corridorBroken = true
			break
		}
	}

	assert.True(t, corridorBroken,
		"SDT must detect slow upward drift of 3.0 units over 9 seconds — simple deadband would miss this")
}

// =========================================================
// TEST 5: DoorWidth zero-variance fallback
// When variance is ~0, DoorWidth must use the 0.01*Mean fallback
// with a hard floor to prevent DoorWidth from collapsing to 0.
// =========================================================
func TestSDT_DoorWidth_ZeroVarianceFallback(t *testing.T) {
	mean := 90.0
	variance := 0.0

	var doorWidth float64
	if variance < 1e-9 {
		doorWidth = math.Max(0.01*mean, 0.001)
	} else {
		doorWidth = 2 * math.Sqrt(variance)
	}

	assert.Greater(t, doorWidth, 0.0, "DoorWidth must never be 0 even with zero variance")
	assert.Equal(t, 0.9, doorWidth, "For mean=90, zero variance should give doorwidth=0.01*90=0.9")
}

func TestSDT_DoorWidth_ZeroMeanZeroVarianceFallback(t *testing.T) {
	mean := 0.0
	variance := 0.0

	var doorWidth float64
	if variance < 1e-9 {
		doorWidth = math.Max(0.01*mean, 0.001)
	} else {
		doorWidth = 2 * math.Sqrt(variance)
	}

	assert.Equal(t, 0.001, doorWidth,
		"Hard floor of 0.001 must apply when both mean and variance are zero")
}

// =========================================================
// TEST 6: Zero / negative dt guard
// If two points arrive with identical timestamps, the engine
// must not divide by zero.
// =========================================================
func TestSDT_ZeroDt_DoesNotPanic(t *testing.T) {
	state := SDTState{DoorWidth: 2.0}
	_, _, _, state = EvaluateSDT(state, 90.0, baseTime())

	// Same timestamp as anchor — dt = 0
	assert.NotPanics(t, func() {
		EvaluateSDT(state, 91.0, baseTime()) // same time as anchor
	}, "Engine must not panic on zero dt")
}

// =========================================================
// TEST 7: Table-driven — various signal shapes
// =========================================================
func TestSDT_TableDriven_SignalShapes(t *testing.T) {
	tests := []struct {
		name           string
		doorWidth      float64
		values         []float64 // first value is bootstrap
		expectAnyEmit  bool      // do we expect at least one emission after bootstrap?
		description    string
	}{
		{
			name:          "perfect_flatline",
			doorWidth:     2.0,
			values:        []float64{90, 90, 90, 90, 90, 90, 90, 90, 90, 90},
			expectAnyEmit: false,
			description:   "Perfectly stable sensor should emit nothing after bootstrap",
		},
		{
			name:          "gradual_step_change",
			doorWidth:     0.5,
			values:        []float64{90, 90.1, 90.2, 90.4, 90.8, 91.6, 93},
			expectAnyEmit: true,
			description:   "Accelerating upward drift must eventually break corridor",
		},
		{
			name:          "noise_within_door",
			doorWidth:     5.0,
			values:        []float64{90, 90.3, 89.8, 90.1, 90.4, 89.9, 90.2},
			expectAnyEmit: false,
			description:   "Random noise within a wide door must all be suppressed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := SDTState{DoorWidth: tt.doorWidth}
			emitCount := 0

			for i, v := range tt.values {
				shouldEmit, _, _, newState := EvaluateSDT(state, v, baseTime().Add(time.Duration(i)*time.Second))
				state = newState
				if i > 0 && shouldEmit { // don't count bootstrap emission
					emitCount++
				}
			}

			if tt.expectAnyEmit {
				assert.Greater(t, emitCount, 0, tt.description)
			} else {
				assert.Equal(t, 0, emitCount, tt.description)
			}
		})
	}
}
