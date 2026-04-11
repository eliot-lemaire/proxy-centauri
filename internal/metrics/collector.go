package metrics

import (
	"net/http"

	dto "github.com/prometheus/client_model/go"

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

// MetricsSnapshot holds a point-in-time summary of metrics for one gate.
type MetricsSnapshot struct {
	Gate     string
	ReqTotal int64
	ErrTotal int64
	P95Ms    float64
}

// Init registers all Centauri metrics with the default Prometheus registry.
func Init() {
	prometheus.MustRegister(RequestsTotal, RequestDuration, ActiveConns, ErrorsTotal)
}

// InitGate pre-initialises zero-value label combinations for a gate so all
// metric families appear in /metrics output even before any traffic arrives.
func InitGate(gate string) {
	RequestsTotal.WithLabelValues(gate, "200").Add(0)
	RequestDuration.WithLabelValues(gate).Observe(0)
	ActiveConns.WithLabelValues(gate).Add(0)
	ErrorsTotal.WithLabelValues(gate, "backend_error").Add(0)
}

// Handler returns the HTTP handler that serves the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Snapshots reads the current metric values from the default Prometheus gatherer
// and returns one MetricsSnapshot per gate name.
func Snapshots(gateNames []string) []MetricsSnapshot {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return nil
	}

	reqTotals := map[string]int64{}
	errTotals := map[string]int64{}
	histograms := map[string]*dto.Histogram{}

	for _, mf := range mfs {
		switch mf.GetName() {
		case "centauri_requests_total":
			for _, m := range mf.GetMetric() {
				gate := labelValue(m.GetLabel(), "gate")
				reqTotals[gate] += int64(m.GetCounter().GetValue())
			}
		case "centauri_errors_total":
			for _, m := range mf.GetMetric() {
				gate := labelValue(m.GetLabel(), "gate")
				errTotals[gate] += int64(m.GetCounter().GetValue())
			}
		case "centauri_request_duration_seconds":
			for _, m := range mf.GetMetric() {
				gate := labelValue(m.GetLabel(), "gate")
				histograms[gate] = m.GetHistogram()
			}
		}
	}

	snaps := make([]MetricsSnapshot, 0, len(gateNames))
	for _, gate := range gateNames {
		snap := MetricsSnapshot{
			Gate:     gate,
			ReqTotal: reqTotals[gate],
			ErrTotal: errTotals[gate],
		}
		if h, ok := histograms[gate]; ok {
			snap.P95Ms = histogramP95(h)
		}
		snaps = append(snaps, snap)
	}
	return snaps
}

// labelValue returns the value of the named label from a label pair slice.
func labelValue(labels []*dto.LabelPair, name string) string {
	for _, lp := range labels {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

// histogramP95 estimates the 95th-percentile latency in milliseconds using
// linear interpolation across the histogram's cumulative bucket counts.
func histogramP95(h *dto.Histogram) float64 {
	total := h.GetSampleCount()
	if total == 0 {
		return 0
	}
	target := 0.95 * float64(total)

	var prevBound float64
	var prevCount uint64

	for _, b := range h.GetBucket() {
		cumCount := b.GetCumulativeCount()
		if float64(cumCount) >= target {
			upperBound := b.GetUpperBound()
			lowerCount := float64(prevCount)
			upperCount := float64(cumCount)
			if upperCount == lowerCount {
				return upperBound * 1000
			}
			p95 := prevBound + (upperBound-prevBound)*(target-lowerCount)/(upperCount-lowerCount)
			return p95 * 1000 // seconds → milliseconds
		}
		prevBound = b.GetUpperBound()
		prevCount = cumCount
	}

	// All observations fell into the +Inf bucket — return the last finite bound.
	return prevBound * 1000
}
