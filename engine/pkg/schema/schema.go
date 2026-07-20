package schema

import (
	"sync"
	"time"
)

type FieldState string

const (
	StateLearned     FieldState = "learned"
	StateConditional FieldState = "conditional"
	StateFlagged     FieldState = "flagged"
	StateLearning    FieldState = "learning"
)

type Baseline struct {
	Count    int
	Sum      float64
	SumSq    float64
	Min      float64
	Max      float64
	Mean     float64
	Variance float64
}

type Field struct {
	Name          string
	Type          string // "float", "string", "bool"
	State         FieldState
	Baseline      Baseline
	DoorWidth     float64
	AnchorValue   float64
	AnchorTime    time.Time
	LastValue     float64
	LastTime      time.Time
	MaxLowerSlope float64
	MinUpperSlope float64
	SDTRestored   bool
	Checklist     Checklist
	DriftType     string
}


type Checklist struct {
	Active          bool // true if still in cold start
	TotalSamples    int  // total readings seen for this sensor
	RequiredSamples int  // how many required to exit (e.g. 50)
	StartTime       time.Time
}

type SensorSchema struct {
	ID                string
	Tier              string // standard, critical, flagged
	Fields            map[string]*Field
	Checklist         Checklist
	LastHeartbeatTime time.Time
	HeartbeatInterval time.Duration
	LastSeenTime             time.Time     // Last time a message was received
	DropoutFlagged           bool          // True if the missing heartbeat flag was already emitted
	ExpectsExternalHeartbeat bool
	WindowDuration           time.Duration // Derived window for fusion
}

type InferResult struct {
	Flags           map[string]string // field_name -> flag type
	DriftPromotions []string          // field names that were promoted this cycle
}

type Registry struct {
	mu      sync.RWMutex
	sensors map[string]*SensorSchema
}

func NewRegistry() *Registry {
	return &Registry{
		sensors: make(map[string]*SensorSchema),
	}
}

func (r *Registry) GetSensor(id string) (*SensorSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, exists := r.sensors[id]
	return s, exists
}

func (r *Registry) GetAllSensors() []*SensorSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sensors []*SensorSchema
	for _, s := range r.sensors {
		sensors = append(sensors, s)
	}
	return sensors
}

func (r *Registry) RegisterSensor(s *SensorSchema) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sensors[s.ID] = s
}
