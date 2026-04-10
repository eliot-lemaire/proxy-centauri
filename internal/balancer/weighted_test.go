package balancer

import "testing"

func TestWeighted_Distribution(t *testing.T) {
	// Weights 3:2:1 — over 6 calls, backend[0] should appear 3x, [1] 2x, [2] 1x.
	w := NewWeighted([]string{"a:1", "b:2", "c:3"}, []int{3, 2, 1})

	counts := map[string]int{}
	for i := 0; i < 6; i++ {
		counts[w.Next()]++
	}

	if counts["a:1"] != 3 {
		t.Errorf("a:1 count = %d, want 3", counts["a:1"])
	}
	if counts["b:2"] != 2 {
		t.Errorf("b:2 count = %d, want 2", counts["b:2"])
	}
	if counts["c:3"] != 1 {
		t.Errorf("c:3 count = %d, want 1", counts["c:3"])
	}
}

func TestWeighted_EqualFallback(t *testing.T) {
	// All weights 0 — should fall back to equal distribution.
	w := NewWeighted([]string{"a:1", "b:2"}, []int{0, 0})

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		counts[w.Next()]++
	}

	// With equal weights, each backend should be selected ~50% of the time.
	// Allow a wide margin since the algorithm is deterministic but we just want
	// both backends to be represented.
	if counts["a:1"] == 0 {
		t.Error("a:1 was never selected with equal weights")
	}
	if counts["b:2"] == 0 {
		t.Error("b:2 was never selected with equal weights")
	}
}

func TestWeighted_SingleBackend(t *testing.T) {
	w := NewWeighted([]string{"only:9000"}, []int{5})
	for i := 0; i < 10; i++ {
		if got := w.Next(); got != "only:9000" {
			t.Errorf("call %d: got %q, want %q", i+1, got, "only:9000")
		}
	}
}

func TestWeighted_Empty(t *testing.T) {
	w := NewWeighted([]string{}, []int{})
	if got := w.Next(); got != "" {
		t.Errorf("Next() on empty = %q, want empty string", got)
	}
}

func TestWeighted_SetBackends(t *testing.T) {
	w := NewWeighted([]string{"a:1", "b:2"}, []int{3, 1})

	// Replace with a new set — should not panic.
	w.SetBackends([]string{"a:1", "c:3"})

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		seen[w.Next()] = true
	}

	if seen["b:2"] {
		t.Error("removed backend b:2 was returned after SetBackends")
	}
	if !seen["a:1"] {
		t.Error("a:1 was never returned after SetBackends")
	}
	if !seen["c:3"] {
		t.Error("c:3 was never returned after SetBackends")
	}
}

func TestWeighted_Len(t *testing.T) {
	w := NewWeighted([]string{"a:1", "b:2", "c:3"}, []int{1, 2, 3})
	if got := w.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	w.SetBackends([]string{"a:1"})
	if got := w.Len(); got != 1 {
		t.Errorf("Len() after SetBackends = %d, want 1", got)
	}
}
