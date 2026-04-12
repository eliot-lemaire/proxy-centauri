package metrics

import (
	"fmt"
	"testing"
)

// newTestStore opens an in-memory SQLite database and initialises the schema.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenStore(t *testing.T) {
	s := newTestStore(t)

	// Verify both tables were created by querying sqlite_master.
	tables := map[string]bool{}
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		tables[name] = true
	}

	for _, want := range []string{"request_stats", "events", "threat_signals"} {
		if !tables[want] {
			t.Errorf("table %q not found after Init", want)
		}
	}
}

func TestFlush_RoundTrip(t *testing.T) {
	s := newTestStore(t)

	snap := MetricsSnapshot{Gate: "web-app", ReqTotal: 10, ErrTotal: 2, P95Ms: 4.5}
	if err := s.Flush(snap); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var ts int64
	var gate string
	var reqTotal, errTotal int64
	var p95Ms float64

	err := s.db.QueryRow(`SELECT ts, gate, req_total, err_total, p95_ms FROM request_stats`).
		Scan(&ts, &gate, &reqTotal, &errTotal, &p95Ms)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	if ts <= 0 {
		t.Errorf("ts = %d, want > 0", ts)
	}
	if gate != snap.Gate {
		t.Errorf("gate = %q, want %q", gate, snap.Gate)
	}
	if reqTotal != snap.ReqTotal {
		t.Errorf("req_total = %d, want %d", reqTotal, snap.ReqTotal)
	}
	if errTotal != snap.ErrTotal {
		t.Errorf("err_total = %d, want %d", errTotal, snap.ErrTotal)
	}
	if p95Ms != snap.P95Ms {
		t.Errorf("p95_ms = %v, want %v", p95Ms, snap.P95Ms)
	}
}

func TestLogEvent_RoundTrip(t *testing.T) {
	s := newTestStore(t)

	if err := s.LogEvent("web-app", "backend_down", ":4000"); err != nil {
		t.Fatalf("LogEvent: %v", err)
	}

	var ts int64
	var gate, kind, detail string

	err := s.db.QueryRow(`SELECT ts, gate, kind, detail FROM events`).
		Scan(&ts, &gate, &kind, &detail)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	if ts <= 0 {
		t.Errorf("ts = %d, want > 0", ts)
	}
	if gate != "web-app" {
		t.Errorf("gate = %q, want %q", gate, "web-app")
	}
	if kind != "backend_down" {
		t.Errorf("kind = %q, want %q", kind, "backend_down")
	}
	if detail != ":4000" {
		t.Errorf("detail = %q, want %q", detail, ":4000")
	}
}

func TestSaveSignal_RoundTrip(t *testing.T) {
	s := newTestStore(t)

	sig := ThreatSignal{
		Gate:      "web-app",
		Kind:      "threat",
		Level:     "high",
		Summary:   "Unusual spike detected",
		Reasoning: "Error rate jumped from 1% to 25% over 60 seconds.",
		Action:    "Set flux_shield to 20 req/s on gate web-app",
	}
	if err := s.SaveSignal(sig); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}

	got, err := s.ListSignals(10)
	if err != nil {
		t.Fatalf("ListSignals: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListSignals returned %d signals, want 1", len(got))
	}

	g := got[0]
	if g.ID <= 0 {
		t.Errorf("ID = %d, want > 0", g.ID)
	}
	if g.Ts <= 0 {
		t.Errorf("Ts = %d, want > 0", g.Ts)
	}
	if g.Gate != sig.Gate {
		t.Errorf("Gate = %q, want %q", g.Gate, sig.Gate)
	}
	if g.Kind != sig.Kind {
		t.Errorf("Kind = %q, want %q", g.Kind, sig.Kind)
	}
	if g.Level != sig.Level {
		t.Errorf("Level = %q, want %q", g.Level, sig.Level)
	}
	if g.Summary != sig.Summary {
		t.Errorf("Summary = %q, want %q", g.Summary, sig.Summary)
	}
	if g.Reasoning != sig.Reasoning {
		t.Errorf("Reasoning = %q, want %q", g.Reasoning, sig.Reasoning)
	}
	if g.Action != sig.Action {
		t.Errorf("Action = %q, want %q", g.Action, sig.Action)
	}
	if g.Resolved {
		t.Error("Resolved = true, want false")
	}
}

func TestListSignals_Limit(t *testing.T) {
	s := newTestStore(t)

	for i := 1; i <= 5; i++ {
		sig := ThreatSignal{
			Gate:      fmt.Sprintf("gate-%d", i),
			Kind:      "threat",
			Level:     "low",
			Summary:   "test",
			Reasoning: "test reasoning",
			Action:    "no action",
		}
		if err := s.SaveSignal(sig); err != nil {
			t.Fatalf("SaveSignal(%d): %v", i, err)
		}
	}

	// Limit respected.
	got, err := s.ListSignals(3)
	if err != nil {
		t.Fatalf("ListSignals(3): %v", err)
	}
	if len(got) != 3 {
		t.Errorf("ListSignals(3) returned %d signals, want 3", len(got))
	}

	// All 5 returned when limit is high enough.
	all, err := s.ListSignals(10)
	if err != nil {
		t.Fatalf("ListSignals(10): %v", err)
	}
	if len(all) != 5 {
		t.Errorf("ListSignals(10) returned %d signals, want 5", len(all))
	}

	// Newest insert (gate-5, highest id) comes first.
	if all[0].Gate != "gate-5" {
		t.Errorf("first signal Gate = %q, want %q", all[0].Gate, "gate-5")
	}
}

func TestResolveSignal(t *testing.T) {
	s := newTestStore(t)

	sig := ThreatSignal{
		Gate:      "web-app",
		Kind:      "scaling",
		Level:     "medium",
		Summary:   "P95 latency elevated",
		Reasoning: "P95 latency is 620ms, above the 500ms threshold.",
		Action:    "Consider adding a second Star System to web-app",
	}
	if err := s.SaveSignal(sig); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}

	before, err := s.ListSignals(10)
	if err != nil {
		t.Fatalf("ListSignals before resolve: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected 1 signal before resolve, got %d", len(before))
	}

	if err := s.ResolveSignal(before[0].ID); err != nil {
		t.Fatalf("ResolveSignal: %v", err)
	}

	after, err := s.ListSignals(10)
	if err != nil {
		t.Fatalf("ListSignals after resolve: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected 0 signals after resolve, got %d", len(after))
	}
}

func TestClose(t *testing.T) {
	s, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
