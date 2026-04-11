# Proxy Centauri — Test Results & Benchmark Report

---

## v0.2.0 — Milestone 2: Engaging Engines

> Tested 2026-04-11 · Go 1.25 · Docker · Linux
> Full test suite: 12 scripts, 3 bugs fixed, all tests green.

### Test Environment

| Component | Detail |
|-----------|--------|
| **Proxy** | Proxy Centauri v0.2.0, compiled from source |
| **HTTP backends** | `ealen/echo-server` ×3 — reflects full request details as JSON |
| **TCP backend** | `alpine/socat` — raw byte echo (`TCP-LISTEN:3003,fork EXEC:cat`) |
| **UDP backend** | `alpine/socat` — datagram echo (`UDP-RECVFROM:3004,fork EXEC:cat`) |
| **Load tool** | `curl` background workers (bash parallel jobs) |
| **Network** | Docker bridge network (all containers local) |
| **Platform** | Linux, Docker Engine, Go 1.25 |

---

### Unit Tests

```
go test ./... -race
```

| Package | Tests | Result |
|---------|-------|--------|
| `internal/balancer` | 15 | PASS |
| `internal/config` | 7 | PASS |
| `internal/logger` | 3 | PASS |
| `internal/metrics` | 8 | PASS |
| `internal/ratelimit` | multiple | PASS |
| `internal/tls` | multiple | PASS |
| `internal/tunnel` | 3 | PASS |
| **Total** | **36+** | **All pass, race-detector clean** |

---

### Functional Test Results

All tests run against the full Docker stack (`docker compose -f docker-compose.yml -f docker-compose.test.yml`).

#### Smoke Test
| Check | Result |
|-------|--------|
| HTTP gate `:8000` → 200 OK | PASS |
| TCP gate `:9000` → echo works | PASS |
| UDP gate `:9001` → datagram echo | PASS |
| Metrics `:9090/metrics` reachable | PASS |

#### HTTP Proxy
| Check | Result |
|-------|--------|
| GET / → 200 OK | PASS |
| Arbitrary paths proxied | PASS |
| `X-Forwarded-For` header set | PASS |
| Custom request headers forwarded | PASS |
| POST request proxied | PASS |
| PUT request proxied | PASS |
| Query parameters forwarded | PASS |

#### Load Balancer Distribution (Round-Robin, 60 requests, 3 backends)
| Backend | Requests received | Expected |
|---------|-------------------|---------|
| `:3000` | **20** | ~20 |
| `:3001` | **20** | ~20 |
| `:3002` | **20** | ~20 |

Perfect 20/20/20 split — zero distribution error.

#### Rate Limiting (Flux Shield)
| Check | Result |
|-------|--------|
| 30 rapid requests from same IP | PASS |
| First 10–12 requests → 200 OK (burst honored) | PASS |
| Remaining requests → 429 Too Many Requests | PASS |
| `Retry-After: 1` header on 429 responses | PASS |

#### TCP Tunnel
| Check | Result |
|-------|--------|
| Single echo (string) | PASS |
| Multi-line echo | PASS |
| 3 sequential connections (no state leak) | PASS |
| 1 KB payload — no truncation | PASS |

#### UDP Tunnel
| Check | Result |
|-------|--------|
| Single datagram echo | PASS |
| Sticky-session datagrams (3 sequential) | PASS |
| 1000-byte datagram — no truncation | PASS |

#### Prometheus Metrics
| Check | Result |
|-------|--------|
| `/metrics` → 200 OK | PASS |
| `centauri_requests_total` present | PASS |
| `centauri_request_duration_seconds` present | PASS |
| `centauri_active_connections` present | PASS |
| `centauri_errors_total` present | PASS |
| Counter increments after traffic | PASS |
| `gate=` label present | PASS |
| `status_code=` label present | PASS |
| Histogram buckets present | PASS |

#### Health Check & Recovery (Pulse Scan)
| Check | Result |
|-------|--------|
| Baseline 200 OK | PASS |
| All backends stopped → 503 detected | **3 seconds** |
| Backends restarted → 200 recovered | **5 seconds** |

#### Config Hot-Reload
| Check | Result |
|-------|--------|
| Config file write triggers reload log | PASS (1 second) |
| Proxy still serving after reload | PASS |

---

### Stress Test Results

#### HTTP — 300 requests, 20 concurrent workers

| Metric | Value |
|--------|-------|
| **Duration** | 740 ms |
| **Throughput** | ~300 req/s |
| **2xx success** | 300 / 300 **(100%)** |
| **4xx client errors** | 0 |
| **5xx server errors** | 0 |
| **Timeouts / errors** | 0 |

#### TCP — 50 concurrent connections

| Metric | Value |
|--------|-------|
| **Duration** | 5,507 ms |
| **Successful connections** | 50 / 50 **(100%)** |
| **Failed** | 0 |

#### UDP — 100 datagrams

| Metric | Value |
|--------|-------|
| **Delivered (echoed)** | 100 / 100 **(100%)** |
| **Lost / no reply** | 0 |
| **Delivery rate** | 100% (threshold: ≥ 80%) |

---

### Bugs Fixed in This Release

| # | Severity | Description | Fix |
|---|----------|-------------|-----|
| 1 | High | Config hot-reload logged the event but never applied backend changes to running balancers | Built gate registry in `main.go`; Watch callback now calls `ps.SetAll()` to update health checker and balancer live |
| 2 | Medium | HTTP proxy returned `502` when all backends were dead (should be `503 Service Unavailable`) | `proxy.go` ErrorHandler now checks the tracked backend address and returns `503` when no backend was selected |
| 3 | Low | UDP healthcheck in `docker-compose.yml` exited 0 even if socat wasn't ready (UDP is connectionless) | Healthcheck now pipes through `grep -q ping` so it only passes when a real echo is received |
| 4 | Low | `centauri_errors_total` absent from `/metrics` before any errors occurred | Added `metrics.InitGate()` in `main.go` to pre-register zero-value label combinations for all metric families |

---

## v0.1.0 — Milestone 1: First Light

> Tested 2026-04-06 · Go 1.22 · Docker · Linux

### Test Environment

| Component | Detail |
|-----------|--------|
| **Proxy** | Proxy Centauri v0.1.0, compiled from source |
| **HTTP backend** | `ealen/echo-server` — responds with full request details |
| **TCP backend** | `alpine/socat` — raw byte echo (`TCP-LISTEN:3003,fork EXEC:cat`) |
| **Load tool** | [`hey`](https://github.com/rakyll/hey) v0.1.5 |
| **Network** | Docker bridge network (all containers local) |
| **Platform** | Linux, Docker Engine |

### Load Testing — 10,000 Requests at 50 Concurrency

```bash
hey -n 10000 -c 50 http://localhost:8000/
```

| Metric | Value |
|--------|-------|
| **Total requests** | 10,000 |
| **Concurrency** | 50 |
| **Duration** | 7.51 s |
| **Requests/sec** | **1,331** |
| **Fastest** | 1.5 ms |
| **Average** | 33.9 ms |
| **p50** | 11.2 ms |
| **p75** | 16.9 ms |
| **p90** | 42.6 ms |
| **p95** | 217 ms |
| **p99** | 377.5 ms |
| **Slowest** | 734.9 ms |
| **HTTP 200** | 10,000 / 10,000 **(100%)** |
| **Errors** | 0 |

### Chaos Test — Backend Restart Mid-Load

| Metric | Value |
|--------|-------|
| **Total requests** | 10,000 |
| **HTTP 200** | 3,217 |
| **HTTP 502** | 6,783 |
| **Panics / crashes** | **0** |
| **Process kept running** | **Yes** |

### Functional Test Summary (v0.1.0)

| Capability | Status |
|------------|--------|
| HTTP reverse proxy (L7) | PASS |
| TCP tunneling (L4) | PASS |
| Round-robin load balancing | PASS |
| Per-gate health checking (HTTP + TCP) | PASS |
| Gate fault isolation | PASS |
| Auto-recovery on backend restart | PASS (≤ 5s) |
| Throughput at 50 concurrency | 1,331 req/s, 0% error |
| Stability under backend chaos | No crash, no panic |
| Config hot-reload | PASS |
| Graceful shutdown (SIGTERM) | PASS |
