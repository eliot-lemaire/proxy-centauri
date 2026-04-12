package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"gopkg.in/yaml.v3"
)

// runWizard is the entry point for `centauri init`.
// It delegates to wizard() using real stdin and the standard output path.
func runWizard() error {
	return wizard(os.Stdin, "centauri.yml")
}

// wizard runs the interactive config wizard, reading from r and writing the
// generated centauri.yml to outPath.  Kept separate from runWizard so tests
// can supply scripted input without touching the real filesystem.
func wizard(r io.Reader, outPath string) error {
	reader := bufio.NewReader(r)

	fmt.Println("  [ Proxy Centauri ] centauri init — Config Wizard")
	fmt.Println("  ─────────────────────────────────────────────────")

	// ── Jump Gate ────────────────────────────────────────────────────────────
	gateName := prompt(reader, "  Jump Gate name (e.g. web-frontend)", "web-app")
	listen := prompt(reader, "  Listen address (e.g. :8080)", ":8080")
	protocol := prompt(reader, "  Protocol [http/tcp/udp] (default: http)", "http")

	// ── Star Systems (backends) ───────────────────────────────────────────────
	var starSystems []config.StarSystem
	for {
		addr := prompt(reader, "  Backend address (e.g. localhost:3000)", "localhost:3000")
		starSystems = append(starSystems, config.StarSystem{Address: addr, Weight: 1})
		more := prompt(reader, "  Add another backend? [y/N]", "n")
		if !strings.EqualFold(strings.TrimSpace(more), "y") {
			break
		}
	}

	// ── Load balancing ────────────────────────────────────────────────────────
	lbAlgo := prompt(reader, "  Load balancing [round_robin/least_connections/weighted] (default: round_robin)", "round_robin")

	// ── Flux Shield ───────────────────────────────────────────────────────────
	var fluxShield config.FluxShieldConfig
	if strings.EqualFold(prompt(reader, "  Enable rate limiting? [y/N]", "n"), "y") {
		rpsStr := prompt(reader, "    → Requests per second (default: 100)", "100")
		burstStr := prompt(reader, "    → Burst (default: 20)", "20")
		rps, _ := strconv.ParseFloat(rpsStr, 64)
		burst, _ := strconv.Atoi(burstStr)
		if rps <= 0 {
			rps = 100
		}
		if burst <= 0 {
			burst = 20
		}
		fluxShield = config.FluxShieldConfig{RequestsPerSecond: rps, Burst: burst}
	}

	// ── TLS ───────────────────────────────────────────────────────────────────
	var tlsCfg config.TLSConfig
	if strings.EqualFold(prompt(reader, "  Enable TLS? [y/N]", "n"), "y") {
		mode := prompt(reader, "    → Mode [auto/manual]", "auto")
		tlsCfg.Mode = mode
		switch mode {
		case "auto":
			tlsCfg.Domain = prompt(reader, "    → Domain (for auto, e.g. example.com)", "")
		case "manual":
			tlsCfg.CertFile = prompt(reader, "    → Cert file path", "cert.pem")
			tlsCfg.KeyFile = prompt(reader, "    → Key file path", "key.pem")
		}
	}

	gate := config.JumpGate{
		Name:          gateName,
		Listen:        listen,
		Protocol:      protocol,
		OrbitalRouter: lbAlgo,
		FluxShield:    fluxShield,
		TLS:           tlsCfg,
		StarSystems:   starSystems,
	}

	// ── Oracle ────────────────────────────────────────────────────────────────
	var oracleCfg config.OracleConfig
	if strings.EqualFold(prompt(reader, "  Enable The Oracle AI? [y/N]", "n"), "y") {
		apiKey := prompt(reader, "    → Claude API key (or press Enter to use $ORACLE_API_KEY)", "${ORACLE_API_KEY}")
		if apiKey == "" {
			apiKey = "${ORACLE_API_KEY}"
		}
		intervalStr := prompt(reader, "    → Check interval in seconds (default: 300)", "300")
		interval, _ := strconv.Atoi(intervalStr)
		if interval <= 0 {
			interval = 300
		}
		oracleCfg = config.OracleConfig{
			Enabled:             true,
			APIKey:              apiKey,
			Model:               "claude-haiku-4-5-20251001",
			IntervalSeconds:     interval,
			ThreatDetection:     true,
			ScalingAdvisor:      true,
			ErrorRateThreshold:  0.05,
			P95LatencyThreshold: 500,
		}
	}

	// ── Build and write config ────────────────────────────────────────────────
	cfg := config.Config{
		MissionControl: config.MissionControlConfig{Port: 8080, Secret: "change-me"},
		JumpGates:      []config.JumpGate{gate},
		Metrics:        config.MetricsConfig{Enabled: true, Port: 9090},
		Oracle:         oracleCfg,
	}

	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Printf("  Writing %s... ", outPath)

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	fmt.Println("Done.")
	fmt.Println("  Run: centauri   (or: docker compose up)")
	return nil
}

// prompt prints msg, reads one line from reader, and returns it trimmed.
// If the user presses Enter with no input, defaultVal is returned.
func prompt(reader *bufio.Reader, msg, defaultVal string) string {
	fmt.Printf("%s: ", msg)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}
