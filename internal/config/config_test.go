package config

import (
	"os"
	"testing"
)

// writeTemp writes yaml content to a temp file and returns its path.
// The caller is responsible for removing it.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "centauri-*.yml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_NewFields_OrbitalRouter(t *testing.T) {
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    orbital_router: "least_connections"
    star_systems:
      - address: "localhost:3000"
        weight: 1
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.JumpGates[0].OrbitalRouter; got != "least_connections" {
		t.Errorf("OrbitalRouter = %q, want %q", got, "least_connections")
	}
}

func TestLoad_NewFields_TLS_Manual(t *testing.T) {
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8443"
    protocol: http
    tls:
      mode: "manual"
      cert_file: "certs/cert.pem"
      key_file: "certs/key.pem"
    star_systems:
      - address: "localhost:3000"
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	tls := cfg.JumpGates[0].TLS
	if tls.Mode != "manual" {
		t.Errorf("TLS.Mode = %q, want %q", tls.Mode, "manual")
	}
	if tls.CertFile != "certs/cert.pem" {
		t.Errorf("TLS.CertFile = %q, want %q", tls.CertFile, "certs/cert.pem")
	}
	if tls.KeyFile != "certs/key.pem" {
		t.Errorf("TLS.KeyFile = %q, want %q", tls.KeyFile, "certs/key.pem")
	}
}

func TestLoad_NewFields_TLS_Auto(t *testing.T) {
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":443"
    protocol: http
    tls:
      mode: "auto"
      domain: "example.com"
    star_systems:
      - address: "localhost:3000"
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	tls := cfg.JumpGates[0].TLS
	if tls.Mode != "auto" {
		t.Errorf("TLS.Mode = %q, want %q", tls.Mode, "auto")
	}
	if tls.Domain != "example.com" {
		t.Errorf("TLS.Domain = %q, want %q", tls.Domain, "example.com")
	}
}

func TestLoad_NewFields_FluxShield(t *testing.T) {
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    flux_shield:
      requests_per_second: 50.0
      burst: 10
    star_systems:
      - address: "localhost:3000"
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	fs := cfg.JumpGates[0].FluxShield
	if fs.RequestsPerSecond != 50.0 {
		t.Errorf("FluxShield.RequestsPerSecond = %v, want 50.0", fs.RequestsPerSecond)
	}
	if fs.Burst != 10 {
		t.Errorf("FluxShield.Burst = %v, want 10", fs.Burst)
	}
}

func TestLoad_NewFields_Metrics(t *testing.T) {
	path := writeTemp(t, `
metrics:
  enabled: true
  port: 9090
jump_gates: []
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled = false, want true")
	}
	if cfg.Metrics.Port != 9090 {
		t.Errorf("Metrics.Port = %v, want 9090", cfg.Metrics.Port)
	}
}

func TestLoad_NewFields_Defaults(t *testing.T) {
	// Minimal config with none of the new fields — old configs must still parse fine.
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    star_systems:
      - address: "localhost:3000"
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	gate := cfg.JumpGates[0]
	if gate.OrbitalRouter != "" {
		t.Errorf("OrbitalRouter = %q, want empty string", gate.OrbitalRouter)
	}
	if gate.TLS.Mode != "" {
		t.Errorf("TLS.Mode = %q, want empty string", gate.TLS.Mode)
	}
	if gate.FluxShield.RequestsPerSecond != 0 {
		t.Errorf("FluxShield.RequestsPerSecond = %v, want 0", gate.FluxShield.RequestsPerSecond)
	}
	if gate.FluxShield.Burst != 0 {
		t.Errorf("FluxShield.Burst = %v, want 0", gate.FluxShield.Burst)
	}
	if cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled = true, want false")
	}
}

func TestLoad_OracleConfig_Enabled(t *testing.T) {
	t.Setenv("TEST_ORACLE_KEY", "sk-test-123")

	path := writeTemp(t, `
oracle:
  enabled: true
  api_key: "${TEST_ORACLE_KEY}"
  model: "claude-haiku-4-5-20251001"
  interval_seconds: 300
  threat_detection: true
  scaling_advisor: true
  error_rate_threshold: 0.05
  p95_latency_threshold: 500
jump_gates: []
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	o := cfg.Oracle
	if !o.Enabled {
		t.Error("Oracle.Enabled = false, want true")
	}
	if o.APIKey != "sk-test-123" {
		t.Errorf("Oracle.APIKey = %q, want %q", o.APIKey, "sk-test-123")
	}
	if o.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Oracle.Model = %q, want %q", o.Model, "claude-haiku-4-5-20251001")
	}
	if o.IntervalSeconds != 300 {
		t.Errorf("Oracle.IntervalSeconds = %v, want 300", o.IntervalSeconds)
	}
	if !o.ThreatDetection {
		t.Error("Oracle.ThreatDetection = false, want true")
	}
	if !o.ScalingAdvisor {
		t.Error("Oracle.ScalingAdvisor = false, want true")
	}
	if o.ErrorRateThreshold != 0.05 {
		t.Errorf("Oracle.ErrorRateThreshold = %v, want 0.05", o.ErrorRateThreshold)
	}
	if o.P95LatencyThreshold != 500 {
		t.Errorf("Oracle.P95LatencyThreshold = %v, want 500", o.P95LatencyThreshold)
	}
}

func TestLoad_OracleConfig_Defaults(t *testing.T) {
	// No oracle: block — existing configs must still parse fine with zero values.
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    star_systems:
      - address: "localhost:3000"
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	o := cfg.Oracle
	if o.Enabled {
		t.Error("Oracle.Enabled = true, want false")
	}
	if o.APIKey != "" {
		t.Errorf("Oracle.APIKey = %q, want empty string", o.APIKey)
	}
	if o.Model != "" {
		t.Errorf("Oracle.Model = %q, want empty string", o.Model)
	}
	if o.IntervalSeconds != 0 {
		t.Errorf("Oracle.IntervalSeconds = %v, want 0", o.IntervalSeconds)
	}
	if o.ErrorRateThreshold != 0 {
		t.Errorf("Oracle.ErrorRateThreshold = %v, want 0", o.ErrorRateThreshold)
	}
	if o.P95LatencyThreshold != 0 {
		t.Errorf("Oracle.P95LatencyThreshold = %v, want 0", o.P95LatencyThreshold)
	}
}

func TestLoad_StarSystem_Weight(t *testing.T) {
	path := writeTemp(t, `
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    star_systems:
      - address: "localhost:3000"
        weight: 3
      - address: "localhost:3001"
        weight: 1
`)
	defer os.Remove(path)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	systems := cfg.JumpGates[0].StarSystems
	if systems[0].Weight != 3 {
		t.Errorf("StarSystems[0].Weight = %v, want 3", systems[0].Weight)
	}
	if systems[1].Weight != 1 {
		t.Errorf("StarSystems[1].Weight = %v, want 1", systems[1].Weight)
	}
}
