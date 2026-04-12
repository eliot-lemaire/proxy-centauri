package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
)

// Oracle is The Oracle AI engine. It runs on a background ticker, analyses
// traffic metrics using the Claude API, and writes Threat Signals to SQLite.
type Oracle struct {
	cfg       config.OracleConfig
	store     *metrics.Store
	client    anthropic.Client
	gateNames []string

	mu        sync.Mutex
	lastSnaps []GateSnapshot // previous round's snapshots — for delta calculation
	lastCall  time.Time      // when Claude was last called
}

// OracleResponse is the structured JSON Claude is instructed to return.
type OracleResponse struct {
	Kind      string `json:"kind"`      // "threat" | "scaling" | "ok"
	Level     string `json:"level"`     // "low" | "medium" | "high" | "critical"
	Gate      string `json:"gate"`      // affected gate name, or "*" for all
	Summary   string `json:"summary"`   // ≤120 chars plain English
	Reasoning string `json:"reasoning"` // explanation paragraph
	Action    string `json:"action"`    // recommended action string
}

// New creates an Oracle from config. Returns nil if Oracle is disabled or has
// no API key — all public methods are nil-safe, so callers need not guard.
func New(cfg config.OracleConfig, store *metrics.Store, gateNames []string) *Oracle {
	if !cfg.Enabled || cfg.APIKey == "" {
		return nil
	}
	if cfg.Model == "" {
		cfg.Model = "claude-haiku-4-5-20251001"
	}
	if cfg.IntervalSeconds == 0 {
		cfg.IntervalSeconds = 300
	}
	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	return &Oracle{cfg: cfg, store: store, client: client, gateNames: gateNames}
}

// newWithClient is used in tests to inject a pre-configured client (e.g. pointing
// at an httptest.Server) without needing a real API key.
func newWithClient(cfg config.OracleConfig, store *metrics.Store, gateNames []string, client anthropic.Client) *Oracle {
	if cfg.Model == "" {
		cfg.Model = "claude-haiku-4-5-20251001"
	}
	if cfg.IntervalSeconds == 0 {
		cfg.IntervalSeconds = 300
	}
	return &Oracle{cfg: cfg, store: store, client: client, gateNames: gateNames}
}

// Start launches the background analysis ticker. Safe to call on a nil Oracle.
func (o *Oracle) Start() {
	if o == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(o.cfg.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			o.analyze()
		}
	}()
}

// Check triggers an immediate analysis if any gate breaches a configured threshold.
// Called by the metrics flush goroutine in main.go after each 30s flush.
// Safe to call on a nil Oracle.
func (o *Oracle) Check(snaps []GateSnapshot) {
	if o == nil {
		return
	}
	o.mu.Lock()
	call := o.shouldCall(snaps)
	o.mu.Unlock()
	if call {
		o.analyze()
	}
}

// analyze is the core loop body: build snapshot, decide, call Claude, save signal.
func (o *Oracle) analyze() {
	o.mu.Lock()
	interval := time.Since(o.lastCall).Seconds()
	if interval <= 0 {
		interval = float64(o.cfg.IntervalSeconds)
	}
	prev := o.lastSnaps
	o.mu.Unlock()

	snaps := BuildSnapshot(o.gateNames, prev, interval)

	o.mu.Lock()
	call := o.shouldCall(snaps)
	o.mu.Unlock()

	if !call {
		return
	}

	resp, err := o.callClaude(snaps)
	if err != nil {
		log.Printf("  [ The Oracle      ] Claude API error: %v", err)
		return
	}

	o.mu.Lock()
	o.lastCall = time.Now()
	o.lastSnaps = snaps
	o.mu.Unlock()

	if resp.Kind == "ok" {
		return // normal traffic — no signal saved
	}

	sig := metrics.ThreatSignal{
		Gate:      resp.Gate,
		Kind:      resp.Kind,
		Level:     resp.Level,
		Summary:   resp.Summary,
		Reasoning: resp.Reasoning,
		Action:    resp.Action,
	}
	if err := o.store.SaveSignal(sig); err != nil {
		log.Printf("  [ The Oracle      ] failed to save signal: %v", err)
	} else {
		log.Printf("  [ The Oracle      ] %s signal [%s] on %q: %s",
			resp.Kind, resp.Level, resp.Gate, resp.Summary)
	}
}

// shouldCall returns true if enough time has elapsed OR any gate breaches a threshold.
// Must be called with o.mu held.
func (o *Oracle) shouldCall(snaps []GateSnapshot) bool {
	if time.Since(o.lastCall) >= time.Duration(o.cfg.IntervalSeconds)*time.Second {
		return true
	}
	for _, s := range snaps {
		if o.cfg.ErrorRateThreshold > 0 && s.ErrorRate > o.cfg.ErrorRateThreshold {
			return true
		}
		if o.cfg.P95LatencyThreshold > 0 && s.P95Ms > o.cfg.P95LatencyThreshold {
			return true
		}
	}
	return false
}

// callClaude sends the formatted prompt to the Claude API and returns the parsed response.
func (o *Oracle) callClaude(snaps []GateSnapshot) (*OracleResponse, error) {
	system, user := o.buildPrompt(snaps)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg, err := o.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     o.cfg.Model,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: system},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return nil, err
	}
	if len(msg.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	var resp OracleResponse
	if err := json.Unmarshal([]byte(msg.Content[0].Text), &resp); err != nil {
		return nil, fmt.Errorf("parse Claude response: %w", err)
	}
	return &resp, nil
}

// buildPrompt formats gate snapshots into system + user prompt strings.
func (o *Oracle) buildPrompt(snaps []GateSnapshot) (system, user string) {
	system = `You are The Oracle, the AI monitoring engine for Proxy Centauri, a reverse proxy.
Analyse the traffic metrics below and detect threats or scaling issues.
Respond with a single valid JSON object — no other text:
{"kind":"threat|scaling|ok","level":"low|medium|high|critical","gate":"<name or *>","summary":"<120 chars max>","reasoning":"<paragraph>","action":"<recommended action>"}
If traffic is normal, return kind "ok" and level "low".`

	var sb strings.Builder
	sb.WriteString("Current traffic snapshot:\n\n")
	sb.WriteString(fmt.Sprintf("%-20s %10s %12s %10s %10s %10s\n",
		"Gate", "Req/s", "ErrorRate%", "P95ms", "ReqDelta", "P95Delta"))
	sb.WriteString(strings.Repeat("-", 75) + "\n")
	for _, s := range snaps {
		sb.WriteString(fmt.Sprintf("%-20s %10.2f %11.1f%% %10.1f %10.2f %10.1f\n",
			s.Gate, s.ReqPerSec, s.ErrorRate*100, s.P95Ms, s.ReqDelta, s.P95Delta))
	}
	return system, sb.String()
}
