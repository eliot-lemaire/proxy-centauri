package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// testMetrics holds isolated Prometheus metrics and their registry for a single test.
type testMetrics struct {
	reg         *prometheus.Registry
	reqTotal    *prometheus.CounterVec
	reqDur      *prometheus.HistogramVec
	activeConns *prometheus.GaugeVec
}

func newTestMetrics(t *testing.T) *testMetrics {
	t.Helper()
	reg := prometheus.NewRegistry()

	reqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_requests_total",
	}, []string{"gate", "status_code"})
	reqDur := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "test_request_duration_seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"gate"})
	activeConns := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_active_connections",
	}, []string{"gate"})

	reg.MustRegister(reqTotal, reqDur, activeConns)
	return &testMetrics{reg: reg, reqTotal: reqTotal, reqDur: reqDur, activeConns: activeConns}
}

func (tm *testMetrics) middleware(gate string) func(http.Handler) http.Handler {
	return newMiddleware(gate, tm.reqTotal, tm.reqDur, tm.activeConns)
}

func counterValue(t *testing.T, c *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.WithLabelValues(labels...).Write(&m); err != nil {
		t.Fatalf("could not read counter: %v", err)
	}
	return m.Counter.GetValue()
}

func gaugeValue(t *testing.T, g *prometheus.GaugeVec, labels ...string) float64 {
	t.Helper()
	var m dto.Metric
	if err := g.WithLabelValues(labels...).Write(&m); err != nil {
		t.Fatalf("could not read gauge: %v", err)
	}
	return m.Gauge.GetValue()
}

// histogramSamples reads the sample count for a histogram from the registry.
func histogramSamples(t *testing.T, reg *prometheus.Registry, metricName, gate string) (count uint64, sum float64) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("registry gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "gate" && lp.GetValue() == gate {
						return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
					}
				}
			}
		}
	}
	return 0, 0
}

func TestMiddlewareStatus200(t *testing.T) {
	tm := newTestMetrics(t)
	handler := tm.middleware("test-gate")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if v := counterValue(t, tm.reqTotal, "test-gate", "200"); v != 1 {
		t.Errorf("requests_total{200} = %v, want 1", v)
	}
}

func TestMiddlewareStatus404(t *testing.T) {
	tm := newTestMetrics(t)
	handler := tm.middleware("gw")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if v := counterValue(t, tm.reqTotal, "gw", "404"); v != 1 {
		t.Errorf("requests_total{404} = %v, want 1", v)
	}
}

func TestMiddlewareDurationObserved(t *testing.T) {
	tm := newTestMetrics(t)
	handler := tm.middleware("slow-gate")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	count, sum := histogramSamples(t, tm.reg, "test_request_duration_seconds", "slow-gate")
	if count != 1 {
		t.Errorf("histogram sample count = %d, want 1", count)
	}
	if sum <= 0 {
		t.Errorf("histogram sum = %v, want > 0", sum)
	}
}

func TestMiddlewareActiveConnsZeroAfterRequest(t *testing.T) {
	tm := newTestMetrics(t)
	handler := tm.middleware("conn-gate")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := gaugeValue(t, tm.activeConns, "conn-gate"); v != 1 {
			t.Errorf("active_connections during request = %v, want 1", v)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if v := gaugeValue(t, tm.activeConns, "conn-gate"); v != 0 {
		t.Errorf("active_connections after request = %v, want 0", v)
	}
}
