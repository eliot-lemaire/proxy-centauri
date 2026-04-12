package oracle

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
)

// SignalsHandler returns an http.Handler serving Oracle Threat Signals.
// Mount the returned handler at any path in main.go.
//
// GET  /oracle/signals                — active signals as a JSON array (newest first)
// POST /oracle/signals/{id}/resolve   — dismiss a signal by ID (returns 204)
func SignalsHandler(store *metrics.Store) http.Handler {
	mux := http.NewServeMux()

	// Exact path: list all active signals.
	mux.HandleFunc("/oracle/signals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		signals, err := store.ListSignals(100)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if signals == nil {
			signals = []metrics.ThreatSignal{} // always return [] not null
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signals)
	})

	// Trailing-slash subtree: handle /oracle/signals/{id}/resolve
	mux.HandleFunc("/oracle/signals/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Remaining path after /oracle/signals/ should be "{id}/resolve"
		trimmed := strings.TrimPrefix(r.URL.Path, "/oracle/signals/")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 || parts[1] != "resolve" {
			http.NotFound(w, r)
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		if err := store.ResolveSignal(id); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent) // 204 — success, no body
	})

	return mux
}
