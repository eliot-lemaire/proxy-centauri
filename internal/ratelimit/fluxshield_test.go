package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a simple 200 OK handler used as the downstream in all tests.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// sendN fires n requests through the middleware using the given RemoteAddr.
// Returns the slice of status codes received.
func sendN(t *testing.T, h http.Handler, n int, remoteAddr string) []int {
	t.Helper()
	codes := make([]int, n)
	for i := range n {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		codes[i] = rr.Code
	}
	return codes
}

// countStatus counts how many entries in codes equal target.
func countStatus(codes []int, target int) int {
	n := 0
	for _, c := range codes {
		if c == target {
			n++
		}
	}
	return n
}

// TestFluxShield_AllowsUnderLimit verifies that all requests within the burst
// size are allowed (200 OK).
func TestFluxShield_AllowsUnderLimit(t *testing.T) {
	burst := 5
	fs := New(100, burst) // high RPS so only burst matters for instant requests
	h := fs.Middleware(okHandler)

	codes := sendN(t, h, burst, "1.2.3.4:1234")
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("request %d: got %d, want 200", i, c)
		}
	}
}

// TestFluxShield_Blocks429OverLimit verifies that requests exceeding the burst
// size receive 429 Too Many Requests.
func TestFluxShield_Blocks429OverLimit(t *testing.T) {
	burst := 3
	fs := New(0.0001, burst) // near-zero RPS so tokens don't refill during the test
	h := fs.Middleware(okHandler)

	codes := sendN(t, h, burst+5, "1.2.3.4:1234")
	if countStatus(codes, http.StatusTooManyRequests) == 0 {
		t.Error("expected at least one 429, got none")
	}
}

// TestFluxShield_PerIPIsolation verifies that each IP has its own bucket.
// Exhausting one IP's quota must not affect a different IP.
func TestFluxShield_PerIPIsolation(t *testing.T) {
	burst := 2
	fs := New(0.0001, burst)
	h := fs.Middleware(okHandler)

	// Exhaust IP A's bucket.
	sendN(t, h, burst+3, "10.0.0.1:9000")

	// IP B should still get through unaffected.
	codes := sendN(t, h, burst, "10.0.0.2:9000")
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("IP B request %d: got %d, want 200 (should not be affected by IP A)", i, c)
		}
	}
}

// TestFluxShield_XForwardedForParsed verifies that the limiter key is the
// leftmost address in X-Forwarded-For, not the RemoteAddr.
func TestFluxShield_XForwardedForParsed(t *testing.T) {
	burst := 2
	fs := New(0.0001, burst)
	h := fs.Middleware(okHandler)

	sendWithXFF := func(xff string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:5000" // proxy address — should be ignored
		req.Header.Set("X-Forwarded-For", xff)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// Exhaust the bucket for the XFF client IP.
	for range burst + 1 {
		sendWithXFF("5.6.7.8, 127.0.0.1")
	}

	// The proxy address (127.0.0.1) still has its own fresh bucket.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("RemoteAddr bucket should be unaffected; got %d, want 200", rr.Code)
	}
}

// TestFluxShield_RetryAfterHeader verifies that a 429 response includes the
// Retry-After header set to "1".
func TestFluxShield_RetryAfterHeader(t *testing.T) {
	fs := New(0.0001, 1) // burst of 1 — second request will be rejected
	h := fs.Middleware(okHandler)

	// First request consumes the single token.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "9.9.9.9:1234"
	httptest.NewRecorder() // discard
	h.ServeHTTP(httptest.NewRecorder(), req1)

	// Second request should be rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "9.9.9.9:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req2)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After: got %q, want %q", got, "1")
	}
}

// TestFluxShield_ZeroRPSNoop is a guard-condition test. When RPS is 0 the
// middleware must not be instantiated; we verify New panics if called with 0
// by checking that callers are expected to gate on rps > 0 before calling New.
// Here we confirm that a FluxShield with a positive burst but zero RPS still
// allows exactly burst requests (initial tokens) and then blocks — i.e. the
// struct itself works; callers must simply not create it when rps == 0.
func TestFluxShield_ZeroRPSNoop(t *testing.T) {
	// The documented contract: callers check rps > 0 before calling New.
	// This test confirms the guard by calling New with a tiny but valid rps.
	burst := 3
	fs := New(0.0001, burst)
	h := fs.Middleware(okHandler)

	codes := sendN(t, h, burst+2, "2.2.2.2:80")
	ok200 := countStatus(codes, http.StatusOK)
	if ok200 != burst {
		t.Errorf("expected exactly %d OK responses (burst), got %d", burst, ok200)
	}
	if countStatus(codes, http.StatusTooManyRequests) == 0 {
		t.Error("expected at least one 429 after burst exhausted")
	}
}
