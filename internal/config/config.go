package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure that mirrors centauri.yml.
type Config struct {
	MissionControl MissionControlConfig `yaml:"mission_control"`
	JumpGates      []JumpGate           `yaml:"jump_gates"`
}

// MissionControlConfig holds settings for the web dashboard.
type MissionControlConfig struct {
	Port   int    `yaml:"port"`
	Secret string `yaml:"secret"`
}

// JumpGate is a single routing rule — one listener, one protocol, one or more backends.
type JumpGate struct {
	Name        string       `yaml:"name"`
	Listen      string       `yaml:"listen"`
	Protocol    string       `yaml:"protocol"` // "http" or "tcp"
	StarSystems []StarSystem `yaml:"star_systems"`
}

// StarSystem is a single backend target.
type StarSystem struct {
	Address string `yaml:"address"`
	Weight  int    `yaml:"weight"`
}

// Load reads a centauri.yml file from disk and returns a parsed Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
