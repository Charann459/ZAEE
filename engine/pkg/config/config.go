package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Engine  EngineConfig   `yaml:"engine"`
	Sensors []SensorConfig `yaml:"sensors"`
}

type EngineConfig struct {
	ColdStart struct {
		AutoVerify bool `yaml:"auto_verify"`
	} `yaml:"cold_start"`
	LOCF struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"locf"`
	Heartbeat struct {
		DefaultInterval string `yaml:"default_interval"`
	} `yaml:"heartbeat"`
	DefaultWindow string `yaml:"default_window"`
}

type SensorConfig struct {
	ID                       string             `yaml:"id"`
	Tier                     string             `yaml:"tier"`
	ExpectsExternalHeartbeat bool               `yaml:"expects_external_heartbeat"`
	Fields                   []FieldConfig      `yaml:"fields"`
	Deadband                 map[string]*float64 `yaml:"deadband"`
	HeartbeatInterval        string             `yaml:"heartbeat_interval"`
}

type FieldConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

// Load reads and parses a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %w", err)
	}

	// Validation
	for _, s := range cfg.Sensors {
		if s.ID == "" {
			return nil, fmt.Errorf("sensor missing id")
		}
		if s.Tier != "critical" && s.Tier != "standard" && s.Tier != "" {
			return nil, fmt.Errorf("sensor %s has invalid tier: %s", s.ID, s.Tier)
		}
	}

	return &cfg, nil
}
