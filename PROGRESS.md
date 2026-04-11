# Centauri — Progress Log

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
