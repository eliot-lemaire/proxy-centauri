# Centauri — Progress Log

## 2026-04-12 (Milestone 3, Step 1 — Oracle config schema)

**Added `oracle:` config block with env-var API key expansion**

- **`internal/config/config.go`**: Added `OracleConfig` struct with 8 fields (`enabled`, `api_key`, `model`, `interval_seconds`, `threat_detection`, `scaling_advisor`, `error_rate_threshold`, `p95_latency_threshold`). Added `Oracle OracleConfig` field to root `Config` struct. Added `os.ExpandEnv()` call in `Load()` so `api_key: "${ORACLE_API_KEY}"` is resolved at startup — no secrets in YAML files needed.
- **`internal/config/config_test.go`**: Added `TestLoad_OracleConfig_Enabled` (all 8 fields + env-var expansion via `t.Setenv`) and `TestLoad_OracleConfig_Defaults` (absent `oracle:` block gives zero values). All 9 config tests pass, race detector clean.
- **`README.md`**: Added `oracle.*` field reference rows to the config table. Updated roadmap — v0.2.0 marked complete, v0.3.0 "Quantum Link Established" section added with step-by-step checklist.

---

## 2026-04-11 (testing + bug fixes + docs)

**Milestone 2 QA pass: full integration test suite, 4 bug fixes, quickstart guide**

### Bug Fixes
- **`internal/proxy/proxy.go`**: ErrorHandler now returns `503 Service Unavailable` (not `502`) when `balancer.Next()` returns `""` (all Star Systems dead) — checked via tracked backend addr on the request context
- **`cmd/centauri/main.go` + `internal/health/pulsescan.go`**: Hot-reload now actually applies `star_systems` changes. Built a `gateRegistry` map before the gate loop; Watch callback iterates new config gates, looks up by name, and calls `ps.SetAll(newAddrs)`. Added `SetAll()` to `PulseScan` — replaces the watched address list, cleans up stale healthy-map entries, marks new addresses healthy, and calls `balancer.SetBackends()` immediately.
- **`docker-compose.yml`**: Fixed UDP healthcheck false-positive. `echo ping | socat - UDP:localhost:3004` exits 0 even when socat isn't ready (UDP is connectionless). Changed to pipe through `grep -q ping` so it only passes when a real echo is received.
- **`internal/metrics/collector.go` + `cmd/centauri/main.go`**: Added `InitGate(gate string)` to pre-register zero-value label combinations for all four metric families. Called from `main.go` for each HTTP gate at startup so all metric families appear in `/metrics` output even before any traffic arrives.

### Test Infrastructure
- **`scripts/lib/common.sh`**: Shared bash library — `pass()`, `fail()`, `warn()`, `info()`, `header()`, `wait_for_port()`, `wait_for_http()`, `summary()` with colored output and exit code
- **`centauri.test.yml`**: 3 HTTP backends, rate limit 5 rps/burst=10, all 3 protocols active
- **`centauri.lb-test.yml`**: weighted (3:2:1) gate on `:8000`, least_connections gate on `:8001`
- **`docker-compose.test.yml`**: Compose overlay adding `echo-http-2` (port 3001) and `echo-http-3` (port 3002)

### Test Scripts (all passing)
- `scripts/smoke-test.sh` — basic reachability of all 4 endpoints
- `scripts/test-http.sh` — HTTP proxy: 7 checks (methods, headers, query params)
- `scripts/test-lb.sh` — round-robin distribution across 3 backends; perfect 20/20/20 on 60 requests
- `scripts/test-ratelimit.sh` — Flux Shield: 429 triggered, Retry-After header verified
- `scripts/test-tcp.sh` — TCP tunnel echo including 1 KB payload
- `scripts/test-udp.sh` — UDP sticky-session echo including 1000-byte datagram
- `scripts/test-metrics.sh` — 10/10 Prometheus checks including counter increment after traffic
- `scripts/test-healthcheck.sh` — all backends killed → 503 detected in 3s → 200 recovered in 5s
- `scripts/test-hotreload.sh` — config file write triggers reload log within 1s
- `scripts/stress-http.sh` — 300 requests, 20 workers: 300 req/s, 100% success
- `scripts/stress-tcp.sh` — 50 concurrent connections: 100% success
- `scripts/stress-udp.sh` — 100 datagrams: 100% delivery

### Documentation
- **`docs/SETUP.md`**: Full technical guide — prerequisites, quick start, complete YAML field reference, all 11 features explained, TLS setup (auto + manual), monitoring (Prometheus + SQLite), test suite usage, troubleshooting
- **`QUICKSTART.md`**: Plain-language guide for non-technical users — explains concepts without jargon, step-by-step Docker setup, common config patterns, command reference
- **`BENCHMARK.md`**: Updated with full v0.2.0 test results table; v0.1.0 results preserved

---

## 2026-04-11 (docker fixes)
- Fixed Dockerfile: bumped builder image from `golang:1.22-alpine` to `golang:1.25-alpine` to match `go.mod` requirement — `go mod download` was failing with "requires go >= 1.25.0"
- Fixed UDP health checker in `internal/health/pulsescan.go`: `ping()` was falling through to `pingHTTP` for UDP backends; added `pingUDP()` method that dials a connected UDP socket, sends `"ping"`, and waits for any reply within 2s; added `"udp"` case to `ping()` switch; echo-udp backend now passes health checks cleanly with no "is dead" errors
- Verified in Docker: all 3 gates healthy, no "dead" log lines after restart

## 2026-04-11 (docker update)
- Updated Docker configuration to cover all features added in Steps 5–7
- **Dockerfile**: added `RUN mkdir -p /app/data /app/logs` in the runtime stage so SQLite store and JSON request logs have writable directories inside the container
- **docker-compose.yml**: exposed port 9090 (Prometheus metrics); exposed 9001/udp (UDP tunnel); switched config mount from `centauri.yml` (local dev addresses) to `centauri.example.yml` (Docker service names); added named volumes `centauri-data` and `centauri-logs` for persistence; added `echo-udp` service (alpine/socat UDP-RECVFROM:3004) with healthcheck to test the UDP tunnel
- **centauri.example.yml**: added `udp-app` jump gate on `:9001` pointing to `echo-udp:3004` so Docker smoke tests cover all three protocols (HTTP, TCP, UDP)
- All 36 tests pass, `go build ./...` clean

## 2026-04-11 (later)
- Milestone 2 Step 7 complete: UDP Tunnel
- Added `internal/tunnel/udp.go`: `UDPTunnel` struct backed by `net.ListenPacket("udp", ...)` — one socket receives all client datagrams; per-client sticky sessions stored in `sync.Map` (clientAddr → `*udpSession`); each new session dials a backend via `balancer.Next()` and spawns a reply-pump goroutine that reads backend responses and writes them back to the client via `conn.WriteTo`; `LoadOrStore` prevents duplicate backend dials under concurrent datagram bursts; sessions idle for 30 s are evicted by a background ticker goroutine
- Added `internal/tunnel/udp_test.go`: 3 integration tests over loopback — `TestUDPTunnel_ForwardAndReply` (single round-trip), `TestUDPTunnel_SessionReuse` (3 datagrams on same connection), `TestUDPTunnel_MultipleClients` (5 concurrent clients); all use a real UDP echo server and a stub `Balancer`; all pass, race-clean
- Wired `"udp"` protocol branch in `main.go` alongside existing `"tcp"` branch: `tunnel.NewUDP(lb).Listen(gate.Listen)` in a goroutine
- No new dependencies — `net` stdlib only
- README: added UDP to tagline, features table, architecture diagram, Quick Start, config example, field reference, new UDP Tunnel `<details>` section, roadmap item marked complete, project structure updated

---

## 2026-04-11
- Milestone 2 Step 6 complete: SQLite Metrics Persistence
- Added `internal/metrics/store.go`: `Store` struct backed by `database/sql` + `modernc.org/sqlite` (pure-Go, no CGo); schema: `request_stats` (ts, gate, req_total, err_total, p95_ms) and `events` (ts, gate, kind, detail); `OpenStore` creates directory + sets `MaxOpenConns(1)` to prevent `SQLITE_BUSY` under concurrent writers; `Init` uses two separate `Exec` calls (modernc driver doesn't support multi-statement exec)
- Added `Snapshots(gateNames)` to `collector.go`: reads `prometheus.DefaultGatherer`, sums counters per gate across all label dimensions, computes p95 latency (ms) from histogram buckets via linear interpolation; `MetricsSnapshot` struct lives alongside it
- Added `SetEventFunc` to `PulseScan` (`pulsescan.go`): non-breaking callback field; fires `backend_up`/`backend_down` events at existing state-transition points
- Wired in `main.go` (inside `cfg.Metrics.Enabled` block): opens store at `data/metrics.db`, starts 30s ticker goroutine flushing `Snapshots()` per gate, hooks `SetEventFunc` per gate into store, logs `config_reload` event from Watch callback; blank-imports `modernc.org/sqlite` in main
- 4 new tests (in-memory SQLite): `TestOpenStore`, `TestFlush_RoundTrip`, `TestLogEvent_RoundTrip`, `TestClose` — all pass; total suite now 36 tests
- Live-verified: backend_down events appear in `events` table within 5s; `request_stats` rows appear after 30s ticker with correct `req_total` and `p95_ms` values
- README: marked Step 6 done in roadmap, updated project structure

---

## 2026-04-10 (later)
- Milestone 2 Step 5 complete: Prometheus Metrics + Stellar Log
- Added `internal/metrics/collector.go`: declares 4 labelled Prometheus metrics — `centauri_requests_total` (counter), `centauri_request_duration_seconds` (histogram, 10 custom buckets), `centauri_active_connections` (gauge), `centauri_errors_total` (counter); `Init()` registers with default registry, `Handler()` returns the `/metrics` scrape endpoint
- Added `internal/metrics/middleware.go`: HTTP middleware that wraps each request — increments active-conn gauge, captures status code via `statusWriter`, observes latency histogram, increments request counter; `newMiddleware` accepts injectable vecs for test isolation
- Added `internal/logger/stellar.go`: JSON request logger (`stellarlog` package); `New(path)` opens/creates `logs/<gate>.log` with auto-mkdir; `Middleware(gate)` writes one JSON line per request with `time`, `gate`, `method`, `path`, `status`, `latency_ms`, `client_ip`; real client IP extracted from `X-Forwarded-For` header with fallback to `RemoteAddr`
- Wired full middleware chain in `main.go` (outermost → innermost): FluxShield → Metrics → StellarLog → ReverseProxy; also wired the previously missing FluxShield middleware; metrics server starts once before gate loop when `metrics.enabled: true`
- Updated `centauri.example.yml` to enable metrics by default (`enabled: true`)
- 7 new tests: 4 metrics middleware tests (status 200/404, histogram observation, active-conn gauge lifecycle) + 3 Stellar Log tests (JSON shape, status 404, X-Forwarded-For parsing) — all pass; total test suite: 32 tests, 0 failures
- Updated README: marked Step 5 done in roadmap, added Prometheus Metrics and Stellar Log feature rows, new `<details>` sections, updated architecture diagram with full middleware stack, expanded field reference table

---

## 2026-04-10
- Milestone 2 Step 2 complete: Balancer interface + new algorithms
- Extracted `balancer.Balancer` interface — proxy, tunnel, and health checker no longer depend on `*RoundRobin` directly
- Added `balancer.NewFromConfig(addrs, weights, algorithm)` factory — selects algorithm from config field
- Added `LeastConn` balancer: routes each request to the backend with fewest active connections; uses `Acquire`/`Release` to track in-flight work
- Added `Weighted` balancer: Nginx smooth weighted round-robin — no burst bias; weights come from `star_systems[].weight` in config
- Added compile-time interface check to `roundrobin.go` (`var _ Balancer = (*RoundRobin)(nil)`)
- Updated `proxy.go` with context-based tracking for LeastConn Acquire/Release on HTTP request lifecycle
- Updated `tunnel.go` with `defer lc.Release(addr)` around TCP connection lifetime
- Updated `main.go` to use `NewFromConfig` and log which algorithm is active per gate
- 15 tests passing (6 LeastConn, 6 Weighted, 3 RoundRobin), race detector clean

---

## 2026-04-08
- Updated README: new slogan, v0.2.0 version in banner, milestone 2 roadmap progress checklist, contributing note bumped to v0.2.0
- Bumped version to v0.2.0 and updated ASCII banner slogan to "Your traffic, your rules, your universe"
- Kicked off Milestone 2 ("Engaging Engines") — Step 1 complete
- Extended config schema in `internal/config/config.go`: added `OrbitalRouter`, `TLS` (mode/domain/cert/key), and `FluxShield` (rps/burst) fields to `JumpGate`; added top-level `MetricsConfig` (enabled/port) to `Config`
- Updated `centauri.example.yml` with all new fields and inline comments explaining each option
- Added `internal/config/config_test.go` with 7 tests covering every new field and a backwards-compatibility default test — all pass

---

## 2026-04-06
- Investigated and fixed bug where "all star systems are dead" log appeared to refer to the whole proxy rather than a specific gate — added `name` field to `PulseScan` so every log line now includes the gate name (e.g. `tcp-app`)
- Fixed `docker-compose.yml`: added `healthcheck` blocks to `echo-http` and `echo-tcp`, and changed `centauri` `depends_on` to `condition: service_healthy` to prevent startup race
- Verified fix: stopping TCP backend (`echo-tcp`) no longer makes HTTP gate (`web-app`) appear down; logs clearly attribute failures to the correct gate
- Stress tested with `hey` (10,000 req @ 50 concurrency): 100% 200s under normal load, no panics under backend restart mid-run

---

- Added `BENCHMARK.md` to repo root — professional capability report with real load test numbers (1,331 req/s, 0% error, fault isolation verified, chaos test, auto-recovery confirmed)

## 2026-04-06 (earlier)
- Created private GitHub repo `eliot-lemaire/proxySite` for the Proxy Centauri pre-launch website
- Made website fully responsive with CSS media queries (tablet ≤768px, mobile ≤600px): countdown scales, hype grid stacks, form stacks vertically, paddings tighten
- Removed stray `Ok` text from top of `index.html` that was rendering on screen

## 2026-04-05
- Updated copyright year from 2024 to 2026 in LICENSE and README
- Fixed logo output: `fmt.Println(logo)` → `fmt.Print(logo)` to avoid extra blank line
