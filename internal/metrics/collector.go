package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centauri_requests_total",
			Help: "Total number of HTTP requests proxied, partitioned by gate and status code.",
		},
		[]string{"gate", "status_code"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "centauri_request_duration_seconds",
			Help:    "HTTP request latency in seconds, partitioned by gate.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"gate"},
	)

	ActiveConns = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "centauri_active_connections",
			Help: "Number of HTTP requests currently being proxied, partitioned by gate.",
		},
		[]string{"gate"},
	)

	ErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "centauri_errors_total",
			Help: "Total number of proxy errors, partitioned by gate and error type.",
		},
		[]string{"gate", "error_type"},
	)
)

// Init registers all Centauri metrics with the default Prometheus registry.
func Init() {
	prometheus.MustRegister(RequestsTotal, RequestDuration, ActiveConns, ErrorsTotal)
}

// Handler returns the HTTP handler that serves the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
