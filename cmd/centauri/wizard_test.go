package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"gopkg.in/yaml.v3"
)

func TestPrompt_Default(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := prompt(r, "Test prompt", "mydefault")
	if got != "mydefault" {
		t.Errorf("got %q, want %q", got, "mydefault")
	}
}

func TestPrompt_Value(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("custom-value\n"))
	got := prompt(r, "Test prompt", "mydefault")
	if got != "custom-value" {
		t.Errorf("got %q, want %q", got, "custom-value")
	}
}

func TestRunWizard_GeneratesValidYAML(t *testing.T) {
	// Scripted answers in prompt order:
	// 1. Gate name
	// 2. Listen address
	// 3. Protocol
	// 4. Backend address
	// 5. Add another backend? → n
	// 6. Load balancing
	// 7. Enable rate limiting? → n
	// 8. Enable TLS? → n
	// 9. Enable The Oracle AI? → n
	input := strings.Join([]string{
		"web-app",       // gate name
		":8080",         // listen address
		"http",          // protocol
		"localhost:3000", // backend
		"n",             // no more backends
		"round_robin",   // load balancing
		"n",             // no rate limiting
		"n",             // no TLS
		"n",             // no Oracle
		"",              // trailing newline
	}, "\n")

	outPath := filepath.Join(t.TempDir(), "centauri.yml")
	if err := wizard(strings.NewReader(input), outPath); err != nil {
		t.Fatalf("wizard: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if len(cfg.JumpGates) != 1 {
		t.Fatalf("jump_gates len = %d, want 1", len(cfg.JumpGates))
	}
	gate := cfg.JumpGates[0]
	if gate.Name != "web-app" {
		t.Errorf("gate.Name = %q, want %q", gate.Name, "web-app")
	}
	if gate.Listen != ":8080" {
		t.Errorf("gate.Listen = %q, want %q", gate.Listen, ":8080")
	}
	if gate.Protocol != "http" {
		t.Errorf("gate.Protocol = %q, want %q", gate.Protocol, "http")
	}
	if len(gate.StarSystems) != 1 {
		t.Fatalf("star_systems len = %d, want 1", len(gate.StarSystems))
	}
	if gate.StarSystems[0].Address != "localhost:3000" {
		t.Errorf("star_system.Address = %q, want %q", gate.StarSystems[0].Address, "localhost:3000")
	}
	if gate.OrbitalRouter != "round_robin" {
		t.Errorf("orbital_router = %q, want %q", gate.OrbitalRouter, "round_robin")
	}
	if cfg.Metrics.Enabled != true {
		t.Error("metrics.enabled should be true")
	}
	if cfg.Metrics.Port != 9090 {
		t.Errorf("metrics.port = %d, want 9090", cfg.Metrics.Port)
	}
	if cfg.Oracle.Enabled {
		t.Error("oracle.enabled should be false when wizard answer is 'n'")
	}
}

func TestRunWizard_WithOracle(t *testing.T) {
	// Same as above but says "y" to Oracle.
	// Prompt order after "n" to TLS:
	// 9. Enable The Oracle AI? → y
	// 10. API key
	// 11. Interval
	input := strings.Join([]string{
		"api-gate",       // gate name
		":9000",          // listen
		"http",           // protocol
		"backend:4000",   // backend
		"n",              // no more backends
		"round_robin",    // lb
		"n",              // no rate limiting
		"n",              // no TLS
		"y",              // YES oracle
		"sk-test-key",    // api key
		"120",            // interval
		"",
	}, "\n")

	outPath := filepath.Join(t.TempDir(), "centauri.yml")
	if err := wizard(strings.NewReader(input), outPath); err != nil {
		t.Fatalf("wizard: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if !cfg.Oracle.Enabled {
		t.Error("oracle.enabled should be true")
	}
	if cfg.Oracle.APIKey != "sk-test-key" {
		t.Errorf("oracle.api_key = %q, want %q", cfg.Oracle.APIKey, "sk-test-key")
	}
	if cfg.Oracle.IntervalSeconds != 120 {
		t.Errorf("oracle.interval_seconds = %d, want 120", cfg.Oracle.IntervalSeconds)
	}
	if cfg.Oracle.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("oracle.model = %q, want %q", cfg.Oracle.Model, "claude-haiku-4-5-20251001")
	}
	if !cfg.Oracle.ThreatDetection {
		t.Error("oracle.threat_detection should be true")
	}
	if !cfg.Oracle.ScalingAdvisor {
		t.Error("oracle.scaling_advisor should be true")
	}
}

func TestRunWizard_MultipleBackends(t *testing.T) {
	// Says "y" to add another backend after the first.
	input := strings.Join([]string{
		"lb-gate",         // gate name
		":8000",           // listen
		"http",            // protocol
		"backend-1:3001",  // first backend
		"y",               // add another
		"backend-2:3002",  // second backend
		"n",               // no more
		"least_connections", // lb
		"n",               // no rate limiting
		"n",               // no TLS
		"n",               // no Oracle
		"",
	}, "\n")

	outPath := filepath.Join(t.TempDir(), "centauri.yml")
	if err := wizard(strings.NewReader(input), outPath); err != nil {
		t.Fatalf("wizard: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	gate := cfg.JumpGates[0]
	if len(gate.StarSystems) != 2 {
		t.Fatalf("star_systems len = %d, want 2", len(gate.StarSystems))
	}
	if gate.StarSystems[0].Address != "backend-1:3001" {
		t.Errorf("star_systems[0] = %q, want %q", gate.StarSystems[0].Address, "backend-1:3001")
	}
	if gate.StarSystems[1].Address != "backend-2:3002" {
		t.Errorf("star_systems[1] = %q, want %q", gate.StarSystems[1].Address, "backend-2:3002")
	}
	if gate.OrbitalRouter != "least_connections" {
		t.Errorf("orbital_router = %q, want %q", gate.OrbitalRouter, "least_connections")
	}
}
