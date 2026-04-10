package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// statusWriter wraps http.ResponseWriter to capture the status code written by the handler.
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

// Middleware returns an HTTP middleware that instruments requests for the named gate
// using the package-level Prometheus metric variables (registered via Init).
func Middleware(gate string) func(http.Handler) http.Handler {
	return newMiddleware(gate, RequestsTotal, RequestDuration, ActiveConns)
}

// newMiddleware is the testable core — accepts explicit metric vecs so tests can
// pass isolated registries without touching the default one.
func newMiddleware(
	gate string,
	reqTotal *prometheus.CounterVec,
	reqDur *prometheus.HistogramVec,
	activeConns *prometheus.GaugeVec,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			activeConns.WithLabelValues(gate).Inc()
			start := time.Now()

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			reqDur.WithLabelValues(gate).Observe(time.Since(start).Seconds())
			reqTotal.WithLabelValues(gate, strconv.Itoa(sw.status)).Inc()
			activeConns.WithLabelValues(gate).Dec()
		})
	}
}
