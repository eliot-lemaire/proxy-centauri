package oracle

import "github.com/eliot-lemaire/proxy-centauri/internal/metrics"

// GateSnapshot is a formatted, rate-based summary of one gate's traffic.
// It is built from raw Prometheus counters and used to construct the Claude prompt.
type GateSnapshot struct {
	Gate      string
	ReqPerSec float64 // requests per second over the last interval
	ErrorRate float64 // fraction of requests that errored (0.0–1.0)
	P95Ms     float64 // 95th-percentile latency in milliseconds
	ReqDelta  float64 // change in req/s vs previous snapshot (positive = more traffic)
	ErrDelta  float64 // change in error rate vs previous snapshot
	P95Delta  float64 // change in P95 ms vs previous snapshot

	// unexported — raw cumulative totals stored so the next round can compute deltas
	rawReqTotal int64
	rawErrTotal int64
}

// BuildSnapshot reads live Prometheus metrics and returns one GateSnapshot per gate.
// prevSnaps is the previous call's result — pass nil on the first call.
// intervalSecs is the seconds elapsed since the last call (used for req/s calculation).
func BuildSnapshot(gateNames []string, prevSnaps []GateSnapshot, intervalSecs float64) []GateSnapshot {
	return buildSnapshot(metrics.Snapshots(gateNames), prevSnaps, intervalSecs)
}

// buildSnapshot is the pure-math core, separated from BuildSnapshot so tests can
// inject known MetricsSnapshot values without touching any Prometheus registry.
func buildSnapshot(current []metrics.MetricsSnapshot, prev []GateSnapshot, intervalSecs float64) []GateSnapshot {
	prevMap := make(map[string]GateSnapshot, len(prev))
	for _, p := range prev {
		prevMap[p.Gate] = p
	}
	if intervalSecs <= 0 {
		intervalSecs = 1 // guard against division by zero
	}

	snaps := make([]GateSnapshot, 0, len(current))
	for _, cur := range current {
		snap := GateSnapshot{
			Gate:        cur.Gate,
			P95Ms:       cur.P95Ms,
			rawReqTotal: cur.ReqTotal,
			rawErrTotal: cur.ErrTotal,
		}

		deltaReq := cur.ReqTotal
		deltaErr := cur.ErrTotal

		if p, ok := prevMap[cur.Gate]; ok {
			deltaReq = cur.ReqTotal - p.rawReqTotal
			deltaErr = cur.ErrTotal - p.rawErrTotal
			if deltaReq < 0 {
				deltaReq = 0 // guard against counter reset
			}
			if deltaErr < 0 {
				deltaErr = 0
			}
		}

		snap.ReqPerSec = float64(deltaReq) / intervalSecs
		if deltaReq > 0 {
			snap.ErrorRate = float64(deltaErr) / float64(deltaReq)
		}

		if p, ok := prevMap[cur.Gate]; ok {
			snap.ReqDelta = snap.ReqPerSec - p.ReqPerSec
			snap.ErrDelta = snap.ErrorRate - p.ErrorRate
			snap.P95Delta = snap.P95Ms - p.P95Ms
		}

		snaps = append(snaps, snap)
	}
	return snaps
}
