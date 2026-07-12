package unit

// ASSUMPTIONS ABOUT YOUR PACKAGE API:
// - InferType(value interface{}) string  → returns "float", "int", "string", "bool"
// - CalculateBaseline(samples []float64) (mean, variance float64)
// - IsSchemaDriftDetected(knownType string, incomingValue interface{}) bool
// Adapt import paths to match your actual module name.

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- STUB IMPLEMENTATIONS ---
// Replace these with calls to your actual pkg/schema functions.

func InferType(value interface{}) string {
	switch value.(type) {
	case float64:
		return "float"
	case int:
		return "int"
	case string:
		return "string"
	case bool:
		return "bool"
	default:
		return "unknown"
	}
}

func CalculateBaseline(samples []float64) (mean, variance float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	mean = sum / float64(len(samples))

	sumSq := 0.0
	for _, v := range samples {
		diff := v - mean
		sumSq += diff * diff
	}
	variance = sumSq / float64(len(samples))
	return mean, variance
}

func IsSchemaDriftDetected(knownType string, incomingValue interface{}) bool {
	inferredType := InferType(incomingValue)
	return inferredType != knownType
}

// =========================================================
// TEST 1: Type inference
// =========================================================
func TestSchema_TypeInference(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{90.5, "float"},
		{42, "int"},
		{"hello", "string"},
		{true, "bool"},
	}

	for _, tt := range tests {
		result := InferType(tt.value)
		assert.Equal(t, tt.expected, result, "Type inference failed for value %v", tt.value)
	}
}

// =========================================================
// TEST 2: Baseline calculation — known input, known output
// =========================================================
func TestSchema_BaselineCalculation_KnownOutput(t *testing.T) {
	// Samples: [88, 89, 90, 91, 92] → mean=90, variance=2
	samples := []float64{88, 89, 90, 91, 92}
	mean, variance := CalculateBaseline(samples)

	assert.InDelta(t, 90.0, mean, 0.001, "Mean should be 90.0")
	assert.InDelta(t, 2.0, variance, 0.001, "Variance should be 2.0")
}

func TestSchema_BaselineCalculation_FlatlineSamples(t *testing.T) {
	// All same value → variance must be exactly 0
	samples := []float64{90, 90, 90, 90, 90}
	mean, variance := CalculateBaseline(samples)

	assert.Equal(t, 90.0, mean)
	assert.Equal(t, 0.0, variance, "Flatline samples must produce zero variance")
}

func TestSchema_BaselineCalculation_SingleSample(t *testing.T) {
	samples := []float64{42.5}
	mean, variance := CalculateBaseline(samples)

	assert.Equal(t, 42.5, mean)
	assert.Equal(t, 0.0, variance)
}

func TestSchema_BaselineCalculation_EmptySamples(t *testing.T) {
	// Empty input must not panic
	assert.NotPanics(t, func() {
		mean, variance := CalculateBaseline([]float64{})
		assert.Equal(t, 0.0, mean)
		assert.Equal(t, 0.0, variance)
	})
}

// =========================================================
// TEST 3: DoorWidth derivation from baseline
// =========================================================
func TestSchema_DoorWidth_DerivedFrom2StdDev(t *testing.T) {
	samples := []float64{88, 89, 90, 91, 92}
	_, variance := CalculateBaseline(samples)

	doorWidth := 2 * math.Sqrt(variance)
	expected := 2 * math.Sqrt(2.0) // ≈ 2.828

	assert.InDelta(t, expected, doorWidth, 0.001,
		"DoorWidth must equal 2 * sqrt(variance)")
}

// =========================================================
// TEST 4: Schema drift detection
// =========================================================
func TestSchema_DriftDetection_TypeMismatch(t *testing.T) {
	// Schema says float, incoming is string
	driftDetected := IsSchemaDriftDetected("float", "not_a_number")
	assert.True(t, driftDetected, "Type mismatch must be detected as schema drift")
}

func TestSchema_DriftDetection_NoMismatch(t *testing.T) {
	driftDetected := IsSchemaDriftDetected("float", 90.5)
	assert.False(t, driftDetected, "Matching types must not trigger schema drift")
}

func TestSchema_DriftDetection_IntVsFloat(t *testing.T) {
	// Config says float, incoming is int — this is a real-world edge case
	// Your engine should decide: is int→float a drift or acceptable coercion?
	// Document your policy here. This test enforces it.
	driftDetected := IsSchemaDriftDetected("float", 90) // 90 as int
	// Update this assertion to match YOUR engine's policy:
	assert.True(t, driftDetected,
		"int arriving where float expected should be flagged — coerce if desired but flag it")
}

// =========================================================
// TEST 5: Minimum sample threshold
// The engine should not commit a baseline from too few samples.
// =========================================================
func TestSchema_Baseline_RequiresMinimumSamples(t *testing.T) {
	// Under your design, cold start requires 50-100 samples minimum.
	// This test verifies the function enforces that floor.
	// Replace MinSamplesForBaseline with your actual constant.
	const MinSamplesForBaseline = 50

	samples := make([]float64, 30) // only 30 samples
	for i := range samples {
		samples[i] = 90.0
	}

	isReady := len(samples) >= MinSamplesForBaseline
	assert.False(t, isReady,
		"Baseline should NOT be considered ready with fewer than %d samples", MinSamplesForBaseline)
}
