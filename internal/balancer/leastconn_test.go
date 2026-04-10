package balancer

import "testing"

func TestLeastConn_PicksLowest(t *testing.T) {
	lc := NewLeastConn([]string{"a:1", "b:2", "c:3"})

	// Simulate a:1 having 2 active connections and b:2 having 1.
	lc.Acquire("a:1")
	lc.Acquire("a:1")
	lc.Acquire("b:2")

	// c:3 has 0 connections — should be selected.
	if got := lc.Next(); got != "c:3" {
		t.Errorf("Next() = %q, want %q", got, "c:3")
	}
}

func TestLeastConn_AcquireRelease(t *testing.T) {
	lc := NewLeastConn([]string{"a:1", "b:2"})

	// Acquire twice on a:1, then release both — count should return to 0.
	lc.Acquire("a:1")
	lc.Acquire("a:1")
	// b:2 has 0 connections → should be picked while a:1 has 2.
	if got := lc.Next(); got != "b:2" {
		t.Errorf("before release: Next() = %q, want %q", got, "b:2")
	}

	lc.Release("a:1")
	lc.Release("a:1")
	// Now both are at 0. Acquire b:2 so a:1 (0) < b:2 (1) → a:1 should win.
	lc.Acquire("b:2")
	if got := lc.Next(); got != "a:1" {
		t.Errorf("after release: Next() = %q, want %q", got, "a:1")
	}
}

func TestLeastConn_SetBackends(t *testing.T) {
	lc := NewLeastConn([]string{"a:1", "b:2", "c:3"})

	// Remove c:3 from the pool.
	lc.SetBackends([]string{"a:1", "b:2"})

	for i := 0; i < 20; i++ {
		got := lc.Next()
		if got == "c:3" {
			t.Errorf("call %d: got removed backend %q", i, got)
		}
	}
}

func TestLeastConn_Empty(t *testing.T) {
	lc := NewLeastConn([]string{})
	if got := lc.Next(); got != "" {
		t.Errorf("Next() on empty = %q, want empty string", got)
	}
}

func TestLeastConn_Len(t *testing.T) {
	lc := NewLeastConn([]string{"a:1", "b:2", "c:3"})
	if got := lc.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	lc.SetBackends([]string{"a:1"})
	if got := lc.Len(); got != 1 {
		t.Errorf("Len() after SetBackends = %d, want 1", got)
	}
}

func TestLeastConn_PreservesCountOnSetBackends(t *testing.T) {
	lc := NewLeastConn([]string{"a:1", "b:2"})
	lc.Acquire("a:1")
	lc.Acquire("a:1")

	// Refresh backends — a:1 stays in pool, should retain its count.
	lc.SetBackends([]string{"a:1", "b:2"})

	// b:2 has 0 connections, a:1 has 2 — b:2 should win.
	if got := lc.Next(); got != "b:2" {
		t.Errorf("Next() = %q, want %q (a:1 count should be preserved)", got, "b:2")
	}
}
