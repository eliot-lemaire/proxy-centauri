package stellarlog

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger writes one JSON line per HTTP request to a log file.
type Logger struct {
	logger *log.Logger
}

// logEntry is the JSON shape written for each request.
type logEntry struct {
	Time      string `json:"time"`
	Gate      string `json:"gate"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	ClientIP  string `json:"client_ip"`
}

// statusWriter captures the status code written by the downstream handler.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wrote {
		sw.status = code
		sw.wrote = true
		sw.ResponseWriter.WriteHeader(code)
	}
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wrote {
		sw.status = http.StatusOK
		sw.wrote = true
	}
	return sw.ResponseWriter.Write(b)
}

// New opens (or creates) the log file at path, creating intermediate directories
// as needed. Returns an error if the file cannot be opened.
func New(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{logger: log.New(f, "", 0)}, nil
}

// NewWithWriter creates a Logger that writes to any io.Writer — useful in tests.
func NewWithWriter(w *log.Logger) *Logger {
	return &Logger{logger: w}
}

// Middleware returns an HTTP middleware that logs one JSON line per request.
func (l *Logger) Middleware(gate string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			entry := logEntry{
				Time:      time.Now().UTC().Format(time.RFC3339),
				Gate:      gate,
				Method:    r.Method,
				Path:      r.URL.Path,
				Status:    sw.status,
				LatencyMs: time.Since(start).Milliseconds(),
				ClientIP:  clientIP(r),
			}
			b, _ := json.Marshal(entry)
			l.logger.Println(string(b))
		})
	}
}

// clientIP extracts the real client IP from X-Forwarded-For, falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; leftmost is the real client
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
