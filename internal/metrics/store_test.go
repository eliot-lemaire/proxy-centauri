package metrics

import (
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

	for _, want := range []string{"request_stats", "events"} {
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
