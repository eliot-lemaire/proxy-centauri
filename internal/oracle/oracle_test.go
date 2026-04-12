package oracle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
)

// newTestStore opens an in-memory SQLite store for use in oracle tests.
func newTestStore(t *testing.T) *metrics.Store {
	t.Helper()
	s, err := metrics.OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newTestOracle builds an Oracle wired to a test store with sensible defaults.
func newTestOracle(t *testing.T, client anthropic.Client) *Oracle {
	t.Helper()
	cfg := config.OracleConfig{
		Enabled:             true,
		APIKey:              "test-key",
		Model:               "claude-haiku-4-5-20251001",
		IntervalSeconds:     300,
		ThreatDetection:     true,
		ScalingAdvisor:      true,
		ErrorRateThreshold:  0.05,
		P95LatencyThreshold: 500,
	}
	return newWithClient(cfg, newTestStore(t), []string{"web-app"}, client)
}

// newMockClaude spins up an httptest.Server that returns a canned Oracle JSON response
// and returns a client pointed at it.
func newMockClaude(t *testing.T, oracleJSON string) (*httptest.Server, anthropic.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Embed the Oracle JSON as the text field in the Anthropic response envelope.
		textJSON, _ := json.Marshal(oracleJSON)
		body := fmt.Sprintf(`{
			"id":"msg_test","type":"message","role":"assistant",
			"content":[{"type":"text","text":%s}],
			"model":"claude-haiku-4-5-20251001","stop_reason":"end_turn",
			"usage":{"input_tokens":10,"output_tokens":50}
		}`, string(textJSON))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(srv.URL),
	)
	t.Cleanup(srv.Close)
	return srv, client
}

// --- Tests ---

func TestOracle_NilSafe(t *testing.T) {
	var o *Oracle
	// Neither call should panic.
	o.Start()
	o.Check(nil)
	o.Check([]GateSnapshot{{Gate: "web-app", ErrorRate: 0.9}})
}

func TestOracle_New_DisabledReturnsNil(t *testing.T) {
	cfg := config.OracleConfig{Enabled: false, APIKey: "sk-test"}
	if o := New(cfg, nil, nil); o != nil {
		t.Error("New() with Enabled=false should return nil")
	}
}

func TestOracle_New_NoKeyReturnsNil(t *testing.T) {
	cfg := config.OracleConfig{Enabled: true, APIKey: ""}
	if o := New(cfg, nil, nil); o != nil {
		t.Error("New() with empty APIKey should return nil")
	}
}

func TestOracle_ShouldCall_Interval(t *testing.T) {
	_, client := newMockClaude(t, `{"kind":"ok","level":"low","gate":"*","summary":"ok","reasoning":"ok","action":"none"}`)
	o := newTestOracle(t, client)

	// Just called — interval not elapsed.
	o.mu.Lock()
	o.lastCall = time.Now()
	o.mu.Unlock()

	o.mu.Lock()
	got := o.shouldCall([]GateSnapshot{})
	o.mu.Unlock()
	if got {
		t.Error("shouldCall() = true immediately after lastCall, want false")
	}

	// Simulate interval elapsed.
	o.mu.Lock()
	o.lastCall = time.Now().Add(-10 * time.Minute)
	o.mu.Unlock()

	o.mu.Lock()
	got = o.shouldCall([]GateSnapshot{})
	o.mu.Unlock()
	if !got {
		t.Error("shouldCall() = false after interval elapsed, want true")
	}
}

func TestOracle_ShouldCall_ErrorRateThreshold(t *testing.T) {
	_, client := newMockClaude(t, `{"kind":"ok","level":"low","gate":"*","summary":"ok","reasoning":"ok","action":"none"}`)
	o := newTestOracle(t, client)

	// Interval NOT elapsed, but error rate is way above threshold.
	o.mu.Lock()
	o.lastCall = time.Now()
	snaps := []GateSnapshot{{Gate: "web-app", ErrorRate: 0.9}}
	got := o.shouldCall(snaps)
	o.mu.Unlock()

	if !got {
		t.Error("shouldCall() = false with ErrorRate=0.9 > threshold=0.05, want true")
	}
}

func TestOracle_ShouldCall_P95Threshold(t *testing.T) {
	_, client := newMockClaude(t, `{"kind":"ok","level":"low","gate":"*","summary":"ok","reasoning":"ok","action":"none"}`)
	o := newTestOracle(t, client)

	// Interval NOT elapsed, but P95 is above threshold.
	o.mu.Lock()
	o.lastCall = time.Now()
	snaps := []GateSnapshot{{Gate: "web-app", P95Ms: 800}}
	got := o.shouldCall(snaps)
	o.mu.Unlock()

	if !got {
		t.Error("shouldCall() = false with P95Ms=800 > threshold=500, want true")
	}
}

func TestOracle_BuildPrompt(t *testing.T) {
	_, client := newMockClaude(t, `{"kind":"ok","level":"low","gate":"*","summary":"ok","reasoning":"ok","action":"none"}`)
	o := newTestOracle(t, client)

	snaps := []GateSnapshot{
		{Gate: "web-app", ReqPerSec: 12.5, ErrorRate: 0.02, P95Ms: 45.0, ReqDelta: 1.2, P95Delta: 3.0},
	}
	system, user := o.buildPrompt(snaps)

	if !strings.Contains(system, "Oracle") {
		t.Error("system prompt missing 'Oracle'")
	}
	if !strings.Contains(system, "JSON") {
		t.Error("system prompt missing 'JSON' instruction")
	}
	if !strings.Contains(user, "web-app") {
		t.Error("user prompt missing gate name 'web-app'")
	}
	if !strings.Contains(user, "Req/s") {
		t.Error("user prompt missing 'Req/s' column header")
	}
	if !strings.Contains(user, "12.50") {
		t.Error("user prompt missing ReqPerSec value '12.50'")
	}
}

func TestOracle_CallClaude_ThreatResponse(t *testing.T) {
	threatJSON := `{"kind":"threat","level":"high","gate":"web-app","summary":"Unusual spike detected","reasoning":"Error rate jumped to 25%.","action":"Set flux_shield to 20 req/s"}`
	_, client := newMockClaude(t, threatJSON)
	o := newTestOracle(t, client)

	snaps := []GateSnapshot{{Gate: "web-app", ReqPerSec: 50, ErrorRate: 0.25, P95Ms: 120}}
	resp, err := o.callClaude(snaps)
	if err != nil {
		t.Fatalf("callClaude: %v", err)
	}

	if resp.Kind != "threat" {
		t.Errorf("Kind = %q, want %q", resp.Kind, "threat")
	}
	if resp.Level != "high" {
		t.Errorf("Level = %q, want %q", resp.Level, "high")
	}
	if resp.Gate != "web-app" {
		t.Errorf("Gate = %q, want %q", resp.Gate, "web-app")
	}
	if resp.Summary != "Unusual spike detected" {
		t.Errorf("Summary = %q, want %q", resp.Summary, "Unusual spike detected")
	}
}

func TestOracle_CallClaude_OkResponse(t *testing.T) {
	okJSON := `{"kind":"ok","level":"low","gate":"*","summary":"Traffic normal","reasoning":"All metrics within expected range.","action":"none"}`
	_, client := newMockClaude(t, okJSON)
	o := newTestOracle(t, client)

	snaps := []GateSnapshot{{Gate: "web-app", ReqPerSec: 5, ErrorRate: 0.01, P95Ms: 30}}
	resp, err := o.callClaude(snaps)
	if err != nil {
		t.Fatalf("callClaude: %v", err)
	}
	if resp.Kind != "ok" {
		t.Errorf("Kind = %q, want %q", resp.Kind, "ok")
	}
}

func TestOracle_ParseResponse(t *testing.T) {
	raw := `{"kind":"scaling","level":"medium","gate":"api","summary":"P95 elevated","reasoning":"P95 at 620ms, above 500ms threshold.","action":"Add a second Star System to api"}`

	var resp OracleResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if resp.Kind != "scaling" {
		t.Errorf("Kind = %q, want %q", resp.Kind, "scaling")
	}
	if resp.Level != "medium" {
		t.Errorf("Level = %q, want %q", resp.Level, "medium")
	}
	if resp.Gate != "api" {
		t.Errorf("Gate = %q, want %q", resp.Gate, "api")
	}
	if resp.Summary != "P95 elevated" {
		t.Errorf("Summary = %q, want %q", resp.Summary, "P95 elevated")
	}
	if resp.Reasoning != "P95 at 620ms, above 500ms threshold." {
		t.Errorf("Reasoning = %q", resp.Reasoning)
	}
	if resp.Action != "Add a second Star System to api" {
		t.Errorf("Action = %q", resp.Action)
	}
}
