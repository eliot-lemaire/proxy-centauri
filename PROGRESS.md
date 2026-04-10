# Centauri — Progress Log

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
