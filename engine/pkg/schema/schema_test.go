package schema

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSchema_TypeInference(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{90.5, "float"},
		{float64(42), "float"}, // Note: JSON unmarshals numbers as float64
		{"hello", "string"},
		{true, "bool"},
	}

	for _, tt := range tests {
		result := InferFieldType(tt.value)
		assert.Equal(t, tt.expected, result, "Type inference failed for value %v", tt.value)
	}
}

func TestSchema_MiniColdStart(t *testing.T) {
	reg := NewRegistry()
	timestamp := time.Now()

	// 1. Register a sensor and artificially complete its main cold start
	reg.Infer("sensor1", map[string]interface{}{"f1": 1.0}, timestamp, nil)
	s, _ := reg.GetSensor("sensor1")
	s.Checklist.Active = false
	s.Checklist.TotalSamples = 50

	// 2. Introduce a new field 'f2'
	res, coldStartActive := reg.Infer("sensor1", map[string]interface{}{"f1": 1.0, "f2": 2.0}, timestamp, nil)
	assert.False(t, coldStartActive, "Main cold start should be false")
	assert.Equal(t, "learning_new_field", res.Flags["f2"], "New field should emit learning flag")
	assert.Len(t, res.DriftPromotions, 0)

	field2 := s.Fields["f2"]
	assert.Equal(t, StateLearning, field2.State)
	assert.True(t, field2.Checklist.Active)
	assert.Equal(t, 0, field2.Checklist.TotalSamples)

	// Feed 25 more samples to complete the mini cold start
	for i := 1; i <= 25; i++ {
		res, _ = reg.Infer("sensor1", map[string]interface{}{"f1": 1.0, "f2": float64(2.0 + i)}, timestamp, nil)
		if i < 25 {
			assert.Equal(t, "learning_new_field", res.Flags["f2"])
		}
	}
	
	// Should be promoted
	assert.Contains(t, res.DriftPromotions, "f2", "f2 should be promoted")
	assert.Equal(t, StateLearned, field2.State)
	assert.False(t, field2.Checklist.Active)
	assert.True(t, field2.DoorWidth > 0, "DoorWidth should be calculated")
}

func TestSchema_DriftAndOscillation(t *testing.T) {
	reg := NewRegistry()
	timestamp := time.Now()

	// Setup sensor and complete main cold start
	reg.Infer("sensor1", map[string]interface{}{"f1": 10.0}, timestamp, nil)
	s, _ := reg.GetSensor("sensor1")
	s.Checklist.Active = false
	s.Checklist.TotalSamples = 50
	s.Fields["f1"].State = StateLearned
	s.Fields["f1"].Type = "float"

	// 1. Introduce drift (float -> string)
	res, _ := reg.Infer("sensor1", map[string]interface{}{"f1": "drifted"}, timestamp, nil)
	assert.Contains(t, res.Flags["f1"], "schema_drift")
	assert.Equal(t, StateFlagged, s.Fields["f1"].State)
	assert.Equal(t, "string", s.Fields["f1"].DriftType)
	assert.Equal(t, 1, s.Fields["f1"].Checklist.TotalSamples)

	// 2. Oscillate drift (string -> bool)
	res, _ = reg.Infer("sensor1", map[string]interface{}{"f1": true}, timestamp, nil)
	assert.Contains(t, res.Flags["f1"], "oscillating")
	assert.Equal(t, "bool", s.Fields["f1"].DriftType)
	assert.Equal(t, 1, s.Fields["f1"].Checklist.TotalSamples, "Checklist should reset to 1 on oscillation")

	// 3. Complete drift to bool
	for i := 2; i <= 25; i++ {
		res, _ = reg.Infer("sensor1", map[string]interface{}{"f1": true}, timestamp, nil)
	}

	assert.Contains(t, res.DriftPromotions, "f1")
	assert.Equal(t, StateLearned, s.Fields["f1"].State)
	assert.Equal(t, "bool", s.Fields["f1"].Type)
	assert.Empty(t, s.Fields["f1"].DriftType)
}

func TestSchema_DriftAbort(t *testing.T) {
	reg := NewRegistry()
	timestamp := time.Now()

	// Setup sensor
	reg.Infer("sensor1", map[string]interface{}{"f1": 10.0}, timestamp, nil)
	s, _ := reg.GetSensor("sensor1")
	s.Checklist.Active = false
	s.Fields["f1"].State = StateLearned

	// Start drift
	reg.Infer("sensor1", map[string]interface{}{"f1": "drifted"}, timestamp, nil)
	assert.Equal(t, StateFlagged, s.Fields["f1"].State)

	// Abort drift (return to original)
	res, _ := reg.Infer("sensor1", map[string]interface{}{"f1": 15.0}, timestamp, nil)
	assert.NotContains(t, res.Flags, "f1")
	assert.Equal(t, StateLearned, s.Fields["f1"].State)
	assert.False(t, s.Fields["f1"].Checklist.Active)
}
