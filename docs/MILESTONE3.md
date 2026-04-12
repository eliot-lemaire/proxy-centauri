# Milestone 3 — The Oracle ("Quantum Link Established")

## Context
Milestone 2 shipped a production-grade proxy with metrics, rate limiting, TLS, and SQLite persistence. Milestone 3 adds The Oracle — an AI brain powered by Claude (Anthropic) that watches live traffic, detects threats and scaling issues, and surfaces smart alerts via a REST endpoint. It also adds `centauri init`, an interactive CLI wizard that writes a working `centauri.yml` without requiring the user to read the docs.

**New external dependency:** `github.com/anthropics/anthropic-sdk-go v1.35.0`

---

## Implementation Order

### Step 1 — `oracle:` Config Block
**File:** `internal/config/config.go`

Added `OracleConfig` struct and wired it into the root `Config`:

```go
type OracleConfig struct {
    Enabled             bool    `yaml:"enabled"`
    APIKey              string  `yaml:"api_key"`               // supports "${ORACLE_API_KEY}"
    Model               string  `yaml:"model"`                 // default: "claude-haiku-4-5-20251001"
    IntervalSeconds     int     `yaml:"interval_seconds"`      // default: 300 (5 min)
    ThreatDetection     bool    `yaml:"threat_detection"`
    ScalingAdvisor      bool    `yaml:"scaling_advisor"`
    ErrorRateThreshold  float64 `yaml:"error_rate_threshold"`  // default: 0.05 (5%)
    P95LatencyThreshold float64 `yaml:"p95_latency_threshold"` // default: 500ms
}
```

`Load()` calls `os.ExpandEnv(cfg.Oracle.APIKey)` after unmarshaling so that
`"${ORACLE_API_KEY}"` is resolved at startup — the raw env-var placeholder
travels safely through YAML without leaking a real key into the config file.

**Tests added:** `TestLoad_OracleConfig_Enabled` (uses `t.Setenv` to verify expansion), `TestLoad_OracleConfig_Defaults` (absent block → zero values).

---

### Step 2 — `threat_signals` SQLite Table
**File:** `internal/metrics/store.go`

New `ThreatSignal` type and three methods on `Store`:

```go
type ThreatSignal struct {
    ID        int64
    Ts        int64
    Gate      string
    Kind      string  // "threat" | "scaling"
    Level     string  // "low" | "medium" | "high" | "critical"
    Summary   string
    Reasoning string
    Action    string
    Resolved  bool
}

func (s *Store) SaveSignal(sig ThreatSignal) error
func (s *Store) ListSignals(limit int) ([]ThreatSignal, error) // WHERE resolved=0 ORDER BY id DESC
func (s *Store) ResolveSignal(id int64) error                  // UPDATE resolved=1
```

Schema added to `Init()`:
```sql
CREATE TABLE IF NOT EXISTS threat_signals (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    ts        INTEGER NOT NULL,
    gate      TEXT NOT NULL,
    kind      TEXT NOT NULL,
    level     TEXT NOT NULL,
    summary   TEXT NOT NULL,
    reasoning TEXT NOT NULL,
    action    TEXT NOT NULL,
    resolved  INTEGER DEFAULT 0
);
```

**Tests added:** `TestSaveSignal_RoundTrip`, `TestListSignals_Limit`, `TestResolveSignal`.

---

### Step 3 — Metrics Aggregator
**New file:** `internal/oracle/aggregator.go`

Converts raw Prometheus cumulative counters into rate-based snapshots for the Claude prompt:

```go
type GateSnapshot struct {
    Gate      string
    ReqPerSec float64 // requests per second over the last interval
    ErrorRate float64 // fraction of requests that errored (0.0–1.0)
    P95Ms     float64 // 95th-percentile latency in ms
    ReqDelta  float64 // change in req/s vs previous snapshot
    ErrDelta  float64 // change in error rate vs previous snapshot
    P95Delta  float64 // change in P95 ms vs previous snapshot
    // unexported raw totals used for delta calculation
    rawReqTotal int64
    rawErrTotal int64
}

func BuildSnapshot(gateNames []string, prevSnaps []GateSnapshot, intervalSecs float64) []GateSnapshot
```

**Design decision:** `BuildSnapshot` is a thin wrapper that calls `metrics.Snapshots()` then delegates to the private `buildSnapshot(current, prev, intervalSecs)` — which takes plain structs. Tests call `buildSnapshot` directly with known values, bypassing the Prometheus registry entirely. Counter resets are handled by flooring negative deltas to 0.

**Tests added:** `TestBuildSnapshot_NoPrev`, `TestBuildSnapshot_WithPrev`, `TestBuildSnapshot_MultiGate`, `TestBuildSnapshot_CounterReset`.

---

### Step 4 — Oracle Core Engine
**New file:** `internal/oracle/oracle.go`

```go
type Oracle struct {
    cfg       config.OracleConfig
    store     *metrics.Store
    client    anthropic.Client  // value type, not pointer
    gateNames []string
    mu        sync.Mutex
    lastSnaps []GateSnapshot
    lastCall  time.Time
}

type OracleResponse struct {
    Kind      string `json:"kind"`      // "threat" | "scaling" | "ok"
    Level     string `json:"level"`     // "low" | "medium" | "high" | "critical"
    Gate      string `json:"gate"`      // affected gate, or "*"
    Summary   string `json:"summary"`   // ≤120 chars
    Reasoning string `json:"reasoning"`
    Action    string `json:"action"`
}

func New(cfg config.OracleConfig, store *metrics.Store, gateNames []string) *Oracle
func (o *Oracle) Start()                     // background ticker — nil-safe
func (o *Oracle) Check(snaps []GateSnapshot) // threshold check — nil-safe
```

Key internal methods:
- `analyze()` — builds snapshot, calls `shouldCall`, calls Claude, saves signal if `kind != "ok"`
- `shouldCall(snaps)` — returns true if interval elapsed **or** any gate exceeds error-rate / P95 threshold (with mutex held)
- `callClaude(snaps)` — 30s context timeout, sends prompt, unmarshals JSON response
- `buildPrompt(snaps)` — formats a traffic table with columns: Gate, Req/s, ErrorRate%, P95ms, ReqDelta, P95Delta

**Design decisions:**
- `anthropic.Client` is a **value type** in the SDK (not a pointer). The struct field and `newWithClient()` parameter are `anthropic.Client`, not `*anthropic.Client`.
- `New()` returns nil when `!cfg.Enabled || cfg.APIKey == ""`. All public methods start with `if o == nil { return }` — callers in `main.go` need no guards.
- `newWithClient()` is an unexported constructor for test injection (bypasses the real `anthropic.NewClient` call).
- When Claude returns `kind: "ok"`, `lastCall` and `lastSnaps` are updated but no signal is saved — conserves API cost.

System prompt sent to Claude:
```
You are The Oracle, the AI monitoring engine for Proxy Centauri, a reverse proxy.
Analyse the traffic metrics below and detect threats or scaling issues.
Respond with a single valid JSON object — no other text:
{"kind":"threat|scaling|ok","level":"low|medium|high|critical","gate":"<name or *>","summary":"<120 chars max>","reasoning":"<paragraph>","action":"<recommended action>"}
If traffic is normal, return kind "ok" and level "low".
```

**Tests added:** 10 tests including `TestOracle_NilSafe`, `TestOracle_ShouldCall_Interval`, `TestOracle_ShouldCall_ErrorRateThreshold`, `TestOracle_ShouldCall_P95Threshold`, `TestOracle_BuildPrompt`, `TestOracle_CallClaude_ThreatResponse`, `TestOracle_CallClaude_OkResponse`, `TestOracle_ParseResponse`.

`newMockClaude()` spins up an `httptest.Server` returning a canned Anthropic response envelope. Tests use `option.WithBaseURL(srv.URL)` to redirect the SDK — no real API calls in tests.

---

### Step 5 — Signals HTTP Endpoint
**New file:** `internal/oracle/signals.go`

```go
func SignalsHandler(store *metrics.Store) http.Handler
```

Uses stdlib `http.ServeMux` with two routes:
- `GET /oracle/signals` — returns up to 100 active signals as JSON array (always `[]`, never `null`)
- `POST /oracle/signals/{id}/resolve` — dismisses a signal, returns 204

Error codes: 405 (wrong method), 400 (non-integer ID), 404 (unknown action word).

**Tests added:** 6 tests — `TestSignalsHandler_GetEmpty`, `TestSignalsHandler_GetWithSignals`, `TestSignalsHandler_Resolve` (full round-trip), `TestSignalsHandler_MethodNotAllowed`, `TestSignalsHandler_InvalidID`, `TestSignalsHandler_NotFound`.

---

### Step 6 — Wire Oracle into main.go
**File:** `cmd/centauri/main.go`

Five changes inside `if cfg.Metrics.Enabled`:

1. `store.Init()` runs before anything else (moved up).
2. Oracle created immediately after store: `ora := oracle.New(cfg.Oracle, store, gateNames); ora.Start()`.
3. Standalone metrics server replaced with a shared `http.ServeMux` mounting `/metrics` (Prometheus) and, when Oracle is non-nil, `/oracle/signals` + `/oracle/signals/` (signals API).
4. Flush goroutine calls `ora.Check(oracle.BuildSnapshot(gateNames, nil, 30))` after each 30s flush — Oracle performs threshold checks right after fresh counters arrive.
5. Version string bumped to `v0.3.0 — Milestone 3: Quantum Link Established`.

`ora.Check()` is nil-safe — no guard needed in the goroutine even when Oracle is disabled.

---

### Step 7 — `centauri init` CLI Wizard
**New file:** `cmd/centauri/wizard.go`

```go
func runWizard() error                               // entry point: os.Stdin → "centauri.yml"
func wizard(r io.Reader, outPath string) error       // testable core
func prompt(reader *bufio.Reader, msg, defaultVal string) string
```

Prompt flow: gate name → listen address → protocol → backend(s) (loop) → load balancing → rate limiting → TLS → Oracle AI. Uses `gopkg.in/yaml.v3` to marshal a `config.Config` struct — no hand-crafted YAML strings.

Entry point added to `main()`:
```go
if len(os.Args) > 1 && os.Args[1] == "init" {
    if err := runWizard(); err != nil {
        log.Fatalf("centauri init failed: %v", err)
    }
    return
}
```

**Design decision:** The testable inner function `wizard(r io.Reader, outPath string)` accepts a `strings.NewReader` in tests and writes to `t.TempDir()` — no real filesystem side effects in the test suite.

**Tests added:** 5 tests — `TestPrompt_Default`, `TestPrompt_Value`, `TestRunWizard_GeneratesValidYAML`, `TestRunWizard_WithOracle`, `TestRunWizard_MultipleBackends`.

---

### Step 8 — Update `centauri.example.yml`
**File:** `centauri.example.yml`

Appended Oracle config block with all eight fields, inline comments, and the `${ORACLE_API_KEY}` env-var placeholder. Defaults to `enabled: false` — opt-in for existing users.

---

## Critical Files

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Edit | `OracleConfig` struct + env-var expansion |
| `internal/metrics/store.go` | Edit | `threat_signals` table + Save/List/Resolve |
| `internal/oracle/aggregator.go` | **Create** | Rate-based snapshot aggregation |
| `internal/oracle/oracle.go` | **Create** | Claude API integration + analysis engine |
| `internal/oracle/signals.go` | **Create** | Signals HTTP endpoint |
| `cmd/centauri/main.go` | Edit | Wire Oracle, shared mux, v0.3.0 bump |
| `cmd/centauri/wizard.go` | **Create** | `centauri init` interactive wizard |
| `centauri.example.yml` | Edit | Oracle config block |

---

## New Dependency

```
github.com/anthropics/anthropic-sdk-go v1.35.0
```

Key SDK notes:
- `anthropic.Client` is a **value type** — do not use `*anthropic.Client`
- `Model` field on `MessageNewParams` is a plain `string`
- `msg.Content[0].Text` is a `string` — no type assertion needed
- Tests use `option.WithBaseURL(srv.URL)` to redirect SDK calls to an `httptest.Server`

---

## Verification

| What | Command | Pass condition |
|------|---------|----------------|
| All tests | `go test ./...` | 64 tests, all green |
| Race detector | `go test -race ./...` | Zero races |
| Binary build | `go build ./cmd/centauri/...` | Exit 0 |
| Wizard | `./centauri init` | Writes valid `centauri.yml` |
| Oracle disabled | Start with no `oracle:` block | No Oracle lines in output, no crash |
| Oracle enabled | `enabled: true` + real API key | Startup shows `[ The Oracle ] AI engine online` |
| Signals endpoint | `curl http://localhost:9090/oracle/signals` | Returns `[]` |
| Metrics endpoint | `curl http://localhost:9090/metrics` | Prometheus output unchanged |
| Signal resolve | `curl -X POST .../oracle/signals/1/resolve` then GET | Signal absent from active list |
| Version | `./centauri` | Logo shows `v0.3.0 — Quantum Link Established` |
