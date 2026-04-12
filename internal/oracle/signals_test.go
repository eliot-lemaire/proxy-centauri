package oracle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
)

// testSignal returns a minimal ThreatSignal for use in signals tests.
func testSignal(gate string) metrics.ThreatSignal {
	return metrics.ThreatSignal{
		Gate:      gate,
		Kind:      "threat",
		Level:     "high",
		Summary:   "Test signal for " + gate,
		Reasoning: "Automated test.",
		Action:    "No action needed.",
	}
}

func TestSignalsHandler_GetEmpty(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/oracle/signals", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Body must be an empty JSON array, not null.
	var got []metrics.ThreatSignal
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestSignalsHandler_GetWithSignals(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	if err := store.SaveSignal(testSignal("gate-a")); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}
	if err := store.SaveSignal(testSignal("gate-b")); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/oracle/signals", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []metrics.ThreatSignal
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestSignalsHandler_Resolve(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	if err := store.SaveSignal(testSignal("web-app")); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}

	// GET to capture the ID.
	req := httptest.NewRequest(http.MethodGet, "/oracle/signals", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var before []metrics.ThreatSignal
	if err := json.NewDecoder(rec.Body).Decode(&before); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected 1 signal before resolve, got %d", len(before))
	}
	id := before[0].ID

	// POST to resolve.
	resolveURL := fmt.Sprintf("/oracle/signals/%d/resolve", id)
	req = httptest.NewRequest(http.MethodPost, resolveURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("resolve status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	// GET again — should be empty.
	req = httptest.NewRequest(http.MethodGet, "/oracle/signals", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var after []metrics.ThreatSignal
	if err := json.NewDecoder(rec.Body).Decode(&after); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected 0 signals after resolve, got %d", len(after))
	}
}

func TestSignalsHandler_MethodNotAllowed(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	// POST to the list endpoint (not a resolve path) should be 405.
	req := httptest.NewRequest(http.MethodPost, "/oracle/signals", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestSignalsHandler_InvalidID(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/oracle/signals/abc/resolve", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSignalsHandler_NotFound(t *testing.T) {
	store := newTestStore(t)
	handler := SignalsHandler(store)

	// Wrong action word — should be 404.
	req := httptest.NewRequest(http.MethodPost, "/oracle/signals/1/dismiss", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
