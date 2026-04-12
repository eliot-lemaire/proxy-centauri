package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure that mirrors centauri.yml.
type Config struct {
	MissionControl MissionControlConfig `yaml:"mission_control"`
	JumpGates      []JumpGate           `yaml:"jump_gates"`
	Metrics        MetricsConfig        `yaml:"metrics"`
	Oracle         OracleConfig         `yaml:"oracle"`
}

// MissionControlConfig holds settings for the web dashboard.
type MissionControlConfig struct {
	Port   int    `yaml:"port"`
	Secret string `yaml:"secret"`
}

// JumpGate is a single routing rule — one listener, one protocol, one or more backends.
type JumpGate struct {
	Name          string          `yaml:"name"`
	Listen        string          `yaml:"listen"`
	Protocol      string          `yaml:"protocol"`       // "http" | "tcp" | "udp"
	OrbitalRouter string          `yaml:"orbital_router"` // "round_robin" | "least_connections" | "weighted"
	TLS           TLSConfig       `yaml:"tls"`
	FluxShield    FluxShieldConfig `yaml:"flux_shield"`
	StarSystems   []StarSystem    `yaml:"star_systems"`
}

// StarSystem is a single backend target.
type StarSystem struct {
	Address string `yaml:"address"`
	Weight  int    `yaml:"weight"`
}

// TLSConfig controls HTTPS for a gate.
type TLSConfig struct {
	Mode     string `yaml:"mode"`      // "auto" (Let's Encrypt) | "manual" | "" (disabled)
	Domain   string `yaml:"domain"`    // required when mode is "auto"
	CertFile string `yaml:"cert_file"` // required when mode is "manual"
	KeyFile  string `yaml:"key_file"`  // required when mode is "manual"
}

// FluxShieldConfig controls per-IP rate limiting for a gate.
type FluxShieldConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"` // 0 = disabled
	Burst             int     `yaml:"burst"`
}

// MetricsConfig controls the Prometheus metrics endpoint.
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"` // default: 9090
}

// OracleConfig controls The Oracle AI engine.
type OracleConfig struct {
	Enabled             bool    `yaml:"enabled"`
	APIKey              string  `yaml:"api_key"`               // supports "${ORACLE_API_KEY}"
	Model               string  `yaml:"model"`                 // default: "claude-haiku-4-5-20251001"
	IntervalSeconds     int     `yaml:"interval_seconds"`      // default: 300 (5 min)
	ThreatDetection     bool    `yaml:"threat_detection"`
	ScalingAdvisor      bool    `yaml:"scaling_advisor"`
	ErrorRateThreshold  float64 `yaml:"error_rate_threshold"`  // default: 0.05 (5%)
	P95LatencyThreshold float64 `yaml:"p95_latency_threshold"` // default: 500ms
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

	cfg.Oracle.APIKey = os.ExpandEnv(cfg.Oracle.APIKey)

	return &cfg, nil
}
