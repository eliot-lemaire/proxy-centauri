package stellarlog

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestLogger creates a Logger that writes to a bytes.Buffer for inspection.
func newTestLogger(buf *bytes.Buffer) *Logger {
	return NewWithWriter(log.New(buf, "", 0))
}

func TestStellarLogWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	handler := l.Middleware("web-app")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no log output written")
	}

	var entry logEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
	}

	if entry.Gate != "web-app" {
		t.Errorf("gate = %q, want %q", entry.Gate, "web-app")
	}
	if entry.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", entry.Method, http.MethodGet)
	}
	if entry.Path != "/api/users" {
		t.Errorf("path = %q, want %q", entry.Path, "/api/users")
	}
	if entry.Status != http.StatusOK {
		t.Errorf("status = %d, want %d", entry.Status, http.StatusOK)
	}
	if entry.Time == "" {
		t.Error("time field is empty")
	}
}

func TestStellarLogStatus404(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	handler := l.Middleware("gate")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry logEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.Status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", entry.Status, http.StatusNotFound)
	}
}

func TestStellarLogClientIPFromXForwardedFor(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf)

	handler := l.Middleware("gate")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry logEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry.ClientIP != "203.0.113.42" {
		t.Errorf("client_ip = %q, want %q", entry.ClientIP, "203.0.113.42")
	}
}
