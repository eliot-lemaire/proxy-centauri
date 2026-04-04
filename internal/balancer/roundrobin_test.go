package balancer

import "testing"

func TestRoundRobin_Cycles(t *testing.T) {
	rr := New([]string{"a:1", "b:2", "c:3"})

	expected := []string{"a:1", "b:2", "c:3", "a:1", "b:2", "c:3"}
	for i, want := range expected {
		got := rr.Next()
		if got != want {
			t.Errorf("call %d: got %q, want %q", i+1, got, want)
		}
	}
}

func TestRoundRobin_SingleBackend(t *testing.T) {
	rr := New([]string{"only:9000"})

	for i := 0; i < 5; i++ {
		got := rr.Next()
		if got != "only:9000" {
			t.Errorf("call %d: got %q, want %q", i+1, got, "only:9000")
		}
	}
}

func TestRoundRobin_Len(t *testing.T) {
	rr := New([]string{"a:1", "b:2"})
	if rr.Len() != 2 {
		t.Errorf("got Len() = %d, want 2", rr.Len())
	}
}
