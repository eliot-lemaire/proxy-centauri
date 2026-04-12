package oracle

import (
	"math"
	"testing"

	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
)

// approxEqual returns true if a and b differ by less than epsilon.
func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestBuildSnapshot_NoPrev(t *testing.T) {
	current := []metrics.MetricsSnapshot{
		{Gate: "web-app", ReqTotal: 100, ErrTotal: 10, P95Ms: 50.0},
	}

	got := buildSnapshot(current, nil, 30)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	s := got[0]

	if s.Gate != "web-app" {
		t.Errorf("Gate = %q, want %q", s.Gate, "web-app")
	}
	if !approxEqual(s.ReqPerSec, 100.0/30.0, 0.001) {
		t.Errorf("ReqPerSec = %v, want ~%v", s.ReqPerSec, 100.0/30.0)
	}
	if !approxEqual(s.ErrorRate, 0.10, 0.001) {
		t.Errorf("ErrorRate = %v, want 0.10", s.ErrorRate)
	}
	if s.P95Ms != 50.0 {
		t.Errorf("P95Ms = %v, want 50.0", s.P95Ms)
	}
	// No prev — all deltas must be zero.
	if s.ReqDelta != 0 {
		t.Errorf("ReqDelta = %v, want 0", s.ReqDelta)
	}
	if s.ErrDelta != 0 {
		t.Errorf("ErrDelta = %v, want 0", s.ErrDelta)
	}
	if s.P95Delta != 0 {
		t.Errorf("P95Delta = %v, want 0", s.P95Delta)
	}
}

func TestBuildSnapshot_WithPrev(t *testing.T) {
	const interval = 30.0

	// Round 1 — no previous data.
	round1 := []metrics.MetricsSnapshot{
		{Gate: "web-app", ReqTotal: 100, ErrTotal: 10, P95Ms: 50.0},
	}
	prev := buildSnapshot(round1, nil, interval)

	// Round 2 — 100 more requests, 5 more errors, P95 jumped to 80ms.
	round2 := []metrics.MetricsSnapshot{
		{Gate: "web-app", ReqTotal: 200, ErrTotal: 15, P95Ms: 80.0},
	}
	got := buildSnapshot(round2, prev, interval)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	s := got[0]

	// deltaReq=100 over 30s → 100/30 ≈ 3.333 req/s
	wantReqPerSec := 100.0 / interval
	if !approxEqual(s.ReqPerSec, wantReqPerSec, 0.001) {
		t.Errorf("ReqPerSec = %v, want ~%v", s.ReqPerSec, wantReqPerSec)
	}

	// deltaErr=5 out of deltaReq=100 → 0.05
	if !approxEqual(s.ErrorRate, 0.05, 0.001) {
		t.Errorf("ErrorRate = %v, want 0.05", s.ErrorRate)
	}

	if s.P95Ms != 80.0 {
		t.Errorf("P95Ms = %v, want 80.0", s.P95Ms)
	}

	// req/s is the same in both rounds → ReqDelta == 0
	if !approxEqual(s.ReqDelta, 0, 0.001) {
		t.Errorf("ReqDelta = %v, want ~0", s.ReqDelta)
	}

	// error rate dropped from 0.10 to 0.05 → ErrDelta == -0.05
	if !approxEqual(s.ErrDelta, -0.05, 0.001) {
		t.Errorf("ErrDelta = %v, want -0.05", s.ErrDelta)
	}

	// P95 rose from 50 to 80 → P95Delta == 30
	if !approxEqual(s.P95Delta, 30.0, 0.001) {
		t.Errorf("P95Delta = %v, want 30.0", s.P95Delta)
	}
}

func TestBuildSnapshot_MultiGate(t *testing.T) {
	current := []metrics.MetricsSnapshot{
		{Gate: "gate-a", ReqTotal: 60, ErrTotal: 6, P95Ms: 10.0},
		{Gate: "gate-b", ReqTotal: 30, ErrTotal: 0, P95Ms: 5.0},
	}

	got := buildSnapshot(current, nil, 60)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	a, b := got[0], got[1]
	if a.Gate != "gate-a" {
		t.Errorf("got[0].Gate = %q, want %q", a.Gate, "gate-a")
	}
	if b.Gate != "gate-b" {
		t.Errorf("got[1].Gate = %q, want %q", b.Gate, "gate-b")
	}

	// gate-a: 60 req / 60s = 1.0 req/s, error rate = 6/60 = 0.10
	if !approxEqual(a.ReqPerSec, 1.0, 0.001) {
		t.Errorf("gate-a ReqPerSec = %v, want 1.0", a.ReqPerSec)
	}
	if !approxEqual(a.ErrorRate, 0.10, 0.001) {
		t.Errorf("gate-a ErrorRate = %v, want 0.10", a.ErrorRate)
	}

	// gate-b: 30 req / 60s = 0.5 req/s, error rate = 0
	if !approxEqual(b.ReqPerSec, 0.5, 0.001) {
		t.Errorf("gate-b ReqPerSec = %v, want 0.5", b.ReqPerSec)
	}
	if b.ErrorRate != 0 {
		t.Errorf("gate-b ErrorRate = %v, want 0", b.ErrorRate)
	}
}

func TestBuildSnapshot_CounterReset(t *testing.T) {
	// Round 1: high totals.
	round1 := []metrics.MetricsSnapshot{
		{Gate: "web-app", ReqTotal: 1000, ErrTotal: 50, P95Ms: 20.0},
	}
	prev := buildSnapshot(round1, nil, 30)

	// Round 2: totals reset to near-zero (process restarted).
	round2 := []metrics.MetricsSnapshot{
		{Gate: "web-app", ReqTotal: 5, ErrTotal: 0, P95Ms: 10.0},
	}
	got := buildSnapshot(round2, prev, 30)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	s := got[0]

	// deltaReq would be 5-1000 = -995; guard floors it to 0.
	if s.ReqPerSec < 0 {
		t.Errorf("ReqPerSec = %v after counter reset, want >= 0", s.ReqPerSec)
	}
	if s.ErrorRate < 0 {
		t.Errorf("ErrorRate = %v after counter reset, want >= 0", s.ErrorRate)
	}
}
