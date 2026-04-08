# Centauri — Progress Log

## 2026-04-08
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
