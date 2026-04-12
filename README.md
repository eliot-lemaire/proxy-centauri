```text

    ██████╗ ██████╗  ██████╗ ██╗  ██╗██╗   ██╗
    ██╔══██╗██╔══██╗██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝
    ██████╔╝██████╔╝██║   ██║ ╚███╔╝  ╚████╔╝
    ██╔═══╝ ██╔══██╗██║   ██║ ██╔██╗   ╚██╔╝
    ██║     ██║  ██║╚██████╔╝██╔╝ ██╗   ██║
    ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝

     ██████╗███████╗███╗   ██╗████████╗ █████╗ ██╗   ██╗██████╗ ██╗
    ██╔════╝██╔════╝████╗  ██║╚══██╔══╝██╔══██╗██║   ██║██╔══██╗██║
    ██║     █████╗  ██╔██╗ ██║   ██║   ███████║██║   ██║██████╔╝██║
    ██║     ██╔══╝  ██║╚██╗██║   ██║   ██╔══██║██║   ██║██╔══██╗██║
    ╚██████╗███████╗██║ ╚████║   ██║   ██║  ██║╚██████╔╝██║  ██║██║
     ╚═════╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝

           ✦  Your traffic, your rules, your universe  ✦
              v0.3.0 — Milestone 3: Quantum Link Established
```

# Proxy Centauri

*A lightweight, config-driven reverse proxy for HTTP, TCP, and UDP — built in Go.*

[![Go 1.25](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white&style=flat-square)](https://golang.org/doc/go1.25)
[![Latest Release](https://img.shields.io/github/v/release/eliot-lemaire/proxy-centauri?style=flat-square)](https://github.com/eliot-lemaire/proxy-centauri/releases)
[![License](https://img.shields.io/github/license/eliot-lemaire/proxy-centauri?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/eliot-lemaire/proxy-centauri?style=flat-square)](https://goreportcard.com/report/github.com/eliot-lemaire/proxy-centauri)
[![Docker Ready](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white&style=flat-square)](https://www.docker.com/)

---
[![Launch Countdown](https://img.shields.io/badge/Proxy%20Centauri-Launch%20Countdown-0A0A0A?style=for-the-badge&logo=rocket&logoColor=white)](https://proxycentauri.com)

## About

Proxy Centauri is a Go reverse proxy that routes HTTP and TCP traffic through a YAML-defined configuration called `centauri.yml`. It is protocol-aware, health-checking, and hot-reloadable — no restarts needed when you update your routes.

Routes are called **Jump Gates**, backends are **Star Systems**, the built-in health checker is **Pulse Scan**, and the active load balancer is the **Orbital Router**.

Ships as a ~10 MB Docker image built from a multi-stage `golang:1.22-alpine` → `alpine:3.19` pipeline. Drop in a `centauri.yml`, run `docker compose up`, and traffic is flowing.

---

## Features

| Feature | Description |
|---|---|
| **L7 HTTP Reverse Proxy** | Forwards HTTP requests via Go's `httputil.ReverseProxy`; preserves real client IP with `X-Forwarded-For`; returns `502` on backend error, `503` when all Star Systems are unreachable |
| **L4 TCP Tunneling** | Bidirectional byte pipe — protocol-agnostic; works for databases, game servers, or any raw TCP service |
| **L4 UDP Tunneling** | Datagram forwarding with per-client sticky sessions; one backend connection per source IP for the duration of the session; idle sessions evicted after 30 s; use cases: DNS, VoIP, game servers |
| **Orbital Router — Round-Robin LB** | Lock-free atomic counter; concurrent-safe; distributes requests evenly across all live Star Systems |
| **Pulse Scan Health Checker** | HTTP: `GET` with 2s timeout (any `< 500` response = alive); TCP: raw dial with 2s timeout; checks every 5s; auto-removes dead backends and restores them on recovery |
| **Config Hot-Reload** | `fsnotify` watches `centauri.yml`; reloads without dropping connections or restarting the process |
| **Multi-Stage Docker Build** | `golang:1.22-alpine` builder → `alpine:3.19` runtime; final image ~10 MB |
| **Graceful Shutdown** | Listens for `SIGINT`/`SIGTERM`; drains cleanly before exit |
| **Flux Shield — Rate Limiting** | Per-IP token-bucket rate limiter; configurable RPS and burst; excess requests receive `429 Too Many Requests` with a `Retry-After: 1` header; idle buckets are evicted after 60 s to prevent memory growth |
| **Stellar Encryption — TLS** | HTTPS for any HTTP Jump Gate; `auto` mode fetches and auto-renews certificates from Let's Encrypt; `manual` mode loads a cert/key pair from disk (self-signed or CA-issued) |
| **Prometheus Metrics** | Scrape-ready `/metrics` endpoint on a configurable port; exposes `centauri_requests_total`, `centauri_request_duration_seconds`, `centauri_active_connections`, and `centauri_errors_total` — label-partitioned per gate |
| **Stellar Log — JSON Logging** | Structured JSON request log written to `logs/<gate>.log` per HTTP gate; fields: `time`, `gate`, `method`, `path`, `status`, `latency_ms`, `client_ip` |

---

## Architecture

```text
  Incoming Traffic
        │
        ├── HTTP :8000 ──▶ ┌──────────────────────────┐
        │                  │     JUMP GATE (HTTP)      │
        ├── TCP  :9000 ──▶ │     JUMP GATE (TCP)       │
        └── UDP  :9001 ──▶ │     JUMP GATE (UDP)       │
                           └────────────┬─────────────┘
                                        │
                           ┌────────────▼─────────────┐
                           │  FLUX SHIELD (rate limit) │
                           └────────────┬─────────────┘
                                        │
                           ┌────────────▼─────────────┐
                           │  METRICS (Prometheus)     │
                           └────────────┬─────────────┘
                                        │
                           ┌────────────▼─────────────┐
                           │  STELLAR LOG (JSON log)   │
                           └────────────┬─────────────┘
                                        │
                             ┌──────────▼──────────┐
                             │    ORBITAL ROUTER   │
                             │   (Load Balancer)   │
                             └────┬──────────┬─────┘
                                  │          │
                     ┌────────────▼──┐  ┌────▼───────────┐
                     │  Star System  │  │  Star System   │
                     │  backend-1    │  │  backend-2     │
                     └───────────────┘  └────────────────┘
                              ▲                ▲
                              └───────┬────────┘
                             ┌────────┴────────┐
                             │   PULSE SCAN    │
                             │  (Health Check) │
                             │   every 5s      │
                             └─────────────────┘

  :9090/metrics ◀── Prometheus scrape
```

---

## Quick Start

### With Docker Compose (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/eliot-lemaire/proxy-centauri.git
cd proxy-centauri

# 2. Optionally edit the config (the defaults work out of the box)
cp centauri.example.yml centauri.yml

# 3. Launch
docker compose up --build
```

Verify it's running:

```bash
# Test the HTTP Jump Gate
curl http://localhost:8000/

# Test the TCP Jump Gate
echo "hello" | nc localhost 9000

# Test the UDP Jump Gate
echo "hello" | nc -u localhost 9001
```

To run in the background: `docker compose up -d --build`
To stop: `docker compose down`

### From Source

> Requires Go 1.22+. `centauri.yml` must exist in the working directory.

```bash
git clone https://github.com/eliot-lemaire/proxy-centauri.git
cd proxy-centauri
go run ./cmd/centauri

# Or build first
go build -o centauri ./cmd/centauri && ./centauri
```

### What You'll See on Boot

```text
  [ Mission Control ] Initializing...
  [ Mission Control ] 2 jump gate(s) configured
  [ Jump Gate       ] "web-app"  →  :8000  (http)
  [ Star System     ]     echo-http:3000
  [ Pulse Scan      ] health checks every 5s
  [ Orbital Router  ] listening on :8000 — ready to route
  [ Jump Gate       ] "tcp-app"  →  :9000  (tcp)
  [ Star System     ]     echo-tcp:3003
  [ Pulse Scan      ] health checks every 5s
  [ Orbital Router  ] listening on :9000 — ready to tunnel
  [ Mission Control ] Ready. Watching for config changes...
```

---

## Configuration

`centauri.yml` is the single source of truth. It is hot-reloaded on save — no restart required.

```yaml
mission_control:
  port: 8080          # Reserved: web dashboard port (not yet active)
  secret: "change-me" # Reserved: dashboard auth secret

jump_gates:

  # HTTP Reverse Proxy
  # Forwards web traffic to one or more Star Systems via httputil.ReverseProxy.
  # Sets X-Forwarded-For on every request.
  # Returns 502 Bad Gateway on backend error; 503 when all Star Systems are unreachable.
  - name: "web-app"         # Unique identifier, shown in logs
    listen: ":8000"          # Bind address — ":8000" = all interfaces on port 8000
    protocol: http           # "http" or "tcp"
    star_systems:
      - address: "myapp:3000"  # host:port — Docker service name, IP, or hostname
        weight: 1              # Reserved for weighted LB (not yet active)
      - address: "myapp:3001"  # Add more Star Systems for load balancing
        weight: 1

  # TCP Tunnel
  # Forwards raw bytes bidirectionally — protocol-agnostic.
  # Works for: MySQL, PostgreSQL, Redis, game servers, MQTT, SSH, anything TCP.
  - name: "db-proxy"
    listen: ":5432"
    protocol: tcp
    star_systems:
      - address: "postgres:5432"

  # UDP Tunnel
  # Forwards datagrams with per-client sticky sessions.
  # Works for: DNS servers, VoIP, game servers, anything UDP.
  - name: "dns-proxy"
    listen: ":5353"
    protocol: udp
    star_systems:
      - address: "dns-backend:53"

# Prometheus metrics endpoint — disabled by default
metrics:
  enabled: true
  port: 9090   # scrape at http://localhost:9090/metrics
```

> [!WARNING]
> Change `mission_control.secret` before deploying to any networked environment. The default value `"change-me"` is not secure.

<details>
<summary>Full field reference</summary>

| Field | Type | Required | Description |
|---|---|---|---|
| `mission_control.port` | int | no | Reserved: future web dashboard port |
| `mission_control.secret` | string | no | Reserved: dashboard auth secret — change before deploying |
| `jump_gates[].name` | string | yes | Unique name, shown in log output |
| `jump_gates[].listen` | string | yes | Bind address, e.g. `:8000` or `127.0.0.1:8080` |
| `jump_gates[].protocol` | string | yes | Transport protocol: `http`, `tcp`, or `udp` |
| `jump_gates[].star_systems[].address` | string | yes | Backend `host:port` — supports DNS names and IPs |
| `jump_gates[].star_systems[].weight` | int | no | Backend weight for weighted round-robin; 0 = equal weight |
| `jump_gates[].orbital_router` | string | no | Load balancer algorithm: `round_robin` (default), `least_connections`, `weighted` |
| `jump_gates[].flux_shield.requests_per_second` | float | no | Token refill rate per client IP; `0` disables Flux Shield |
| `jump_gates[].flux_shield.burst` | int | no | Max burst tokens above the rate |
| `jump_gates[].tls.mode` | string | no | `""` (plain HTTP), `"auto"` (Let's Encrypt), or `"manual"` |
| `jump_gates[].tls.domain` | string | conditional | Required when `tls.mode` is `"auto"` |
| `jump_gates[].tls.cert_file` | string | conditional | Required when `tls.mode` is `"manual"` |
| `jump_gates[].tls.key_file` | string | conditional | Required when `tls.mode` is `"manual"` |
| `metrics.enabled` | bool | no | Expose Prometheus `/metrics` endpoint (default `false`) |
| `metrics.port` | int | no | Port for the metrics server (default `9090`) |
| `oracle.enabled` | bool | no | Activate The Oracle AI engine (default `false`) |
| `oracle.api_key` | string | conditional | Anthropic API key; supports `"${ORACLE_API_KEY}"` env-var syntax |
| `oracle.model` | string | no | Claude model to use (default `"claude-haiku-4-5-20251001"`) |
| `oracle.interval_seconds` | int | no | How often The Oracle analyzes traffic in seconds (default `300`) |
| `oracle.threat_detection` | bool | no | Detect traffic anomalies and threats (default `false`) |
| `oracle.scaling_advisor` | bool | no | Detect when you need to scale up or down (default `false`) |
| `oracle.error_rate_threshold` | float | no | Trigger an immediate Oracle check if error rate exceeds this value, e.g. `0.05` = 5% |
| `oracle.p95_latency_threshold` | float | no | Trigger an immediate Oracle check if P95 latency exceeds this value in ms, e.g. `500` |

</details>

---

## How It Works

<details>
<summary>Pulse Scan — Health Checking</summary>

Pulse Scan runs a background health check loop every 5 seconds per Jump Gate.

- **HTTP backends:** Sends a `GET /` with a 2-second timeout. Any HTTP response below status `500` is considered alive.
- **TCP backends:** Attempts a raw `net.DialTimeout` with a 2-second timeout. A successful connection is considered alive.

When a backend fails a check it is immediately removed from the Orbital Router's active pool. When it recovers, it is automatically restored. If all Star Systems go offline, Pulse Scan logs a warning and new HTTP connections receive `503 Service Unavailable` until at least one backend recovers.

</details>

<details>
<summary>Orbital Router — Load Balancing</summary>

The Orbital Router uses a round-robin algorithm to distribute requests across all live Star Systems.

- **Lock-free hot path:** A `sync/atomic` `uint64` counter is incremented per request — no mutex on the request path.
- **Safe backend updates:** A `sync.RWMutex` wraps the backend slice, allowing Pulse Scan to add/remove backends while requests are in-flight.
- **Empty pool handling:** `Next()` returns an empty string safely when no backends are available, triggering a `503` at the proxy layer.

</details>

<details>
<summary>Flux Shield — Rate Limiting</summary>

Flux Shield is a per-IP token-bucket middleware that sits in front of the HTTP proxy.

Each client IP gets its own bucket, filled at `requests_per_second` tokens per second and capped at `burst` tokens. Every request costs one token. When the bucket is empty the request is rejected with **429 Too Many Requests** and a `Retry-After: 1` header — the backend never sees it.

Buckets that receive no traffic for 60 seconds are evicted automatically to prevent unbounded memory use.

Flux Shield is only active when `flux_shield.requests_per_second > 0` in the gate config:

```yaml
jump_gates:
  - name: "web-app"
    listen: ":8000"
    protocol: http
    flux_shield:
      requests_per_second: 10   # token refill rate
      burst: 20                  # max tokens (concurrent burst size)
    star_systems:
      - address: "myapp:3000"
```

> Setting `requests_per_second: 0` (or omitting the block) disables Flux Shield entirely — zero overhead.

</details>

<details>
<summary>Stellar Encryption — TLS / HTTPS</summary>

Stellar Encryption makes any HTTP Jump Gate serve HTTPS. Two modes are supported:

**`auto` — Let's Encrypt (zero-config certs)**

Proxy Centauri uses the ACME protocol to automatically obtain and renew a certificate for your domain. Certificates are cached in a `.certs/` directory. Port 80 must be reachable from the internet for the HTTP-01 challenge.

```yaml
jump_gates:
  - name: "web-app"
    listen: ":443"
    protocol: http
    tls:
      mode: auto
      domain: "yourdomain.com"   # must resolve to this server's IP
    star_systems:
      - address: "myapp:3000"
```

**`manual` — Bring your own cert**

Load a certificate and private key from disk. Ideal for self-signed certs in local development or certificates issued by an internal CA.

```yaml
jump_gates:
  - name: "web-app"
    listen: ":8443"
    protocol: http
    tls:
      mode: manual
      cert_file: "cert.pem"
      key_file: "key.pem"
    star_systems:
      - address: "myapp:3000"
```

Generate a self-signed cert for local testing:

```bash
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes \
  -subj "/CN=localhost"
curl -k https://localhost:8443/
```

Omitting the `tls` block (or leaving `mode` blank) keeps the gate on plain HTTP — no overhead.

</details>

<details>
<summary>Prometheus Metrics — Observability</summary>

When `metrics.enabled: true` in `centauri.yml`, Proxy Centauri starts a Prometheus-compatible scrape endpoint at `http://localhost:<port>/metrics` (default `:9090`).

**Exposed metrics:**

| Metric | Type | Labels | Description |
|---|---|---|---|
| `centauri_requests_total` | Counter | `gate`, `status_code` | Total proxied requests |
| `centauri_request_duration_seconds` | Histogram | `gate` | End-to-end latency with 10 custom buckets |
| `centauri_active_connections` | Gauge | `gate` | Requests currently in-flight |
| `centauri_errors_total` | Counter | `gate`, `error_type` | Proxy-level errors |

**Config:**

```yaml
metrics:
  enabled: true
  port: 9090   # scrape at http://localhost:9090/metrics
```

**Verify:**

```bash
curl http://localhost:9090/metrics
# centauri_requests_total{gate="web-app",status_code="200"} 42
```

</details>

<details>
<summary>Stellar Log — JSON Request Logging</summary>

Every HTTP Jump Gate writes one JSON line per request to `logs/<gate-name>.log`. The log file is created automatically (including the `logs/` directory) on startup.

**Example log line:**

```json
{"time":"2026-04-10T12:00:00Z","gate":"web-app","method":"GET","path":"/api/users","status":200,"latency_ms":4,"client_ip":"203.0.113.42"}
```

**Fields:**

| Field | Description |
|---|---|
| `time` | RFC 3339 UTC timestamp |
| `gate` | Jump Gate name from config |
| `method` | HTTP method |
| `path` | Request path |
| `status` | HTTP response status code |
| `latency_ms` | Total request duration in milliseconds |
| `client_ip` | Real client IP — extracted from `X-Forwarded-For` if present, falls back to `RemoteAddr` |

**Watch live:**

```bash
tail -f logs/web-app.log
```

</details>

<details>
<summary>UDP Tunneling — Datagram Forwarding</summary>

UDP is stateless — there are no connections, only individual datagrams. Proxy Centauri handles this by maintaining a **sticky session table**: the first datagram from a client IP dials a backend (chosen by the Orbital Router) and stores that connection in a `sync.Map`. All subsequent datagrams from the same source reuse that connection, and backend replies are sent back to the original client.

**Session lifecycle:**
- New source IP → `balancer.Next()` picks a backend → backend connection dialed → session stored
- Existing source IP → session looked up, datagram forwarded immediately
- Sessions idle for **30 seconds** are closed and evicted automatically

**Use cases:** DNS servers, VoIP gateways, game servers, QUIC-over-UDP, time sync (NTP), IoT sensors.

**Config:**

```yaml
jump_gates:
  - name: "dns-proxy"
    listen: ":5353"
    protocol: udp
    star_systems:
      - address: "dns-backend:53"
```

**Verify:**

```bash
echo "hello" | nc -u localhost 5353
# backend receives the datagram; reply comes back to the client
```

> UDP gates do not support Flux Shield, Stellar Log, TLS, or Prometheus middleware — those are HTTP-layer features. The Orbital Router (all three algorithms) and Pulse Scan health checking work normally.

</details>

<details>
<summary>Config Hot-Reload</summary>

`fsnotify` watches `centauri.yml` for `Write` and `Create` events. On change:

1. The YAML is re-parsed and validated.
2. The new config is logged: `[ Config ] Reloaded — N jump gate(s)`.
3. An `onChange` callback is invoked with the new config struct.

> [!NOTE]
> In v0.1.0, hot-reload updates the config in memory and notifies the application, but does not restart active listeners. Changes to `listen` addresses or new Jump Gates take effect on the next process restart. Dynamic listener restart is planned for a future release.

</details>

---

## Roadmap

**v0.1.0 — First Contact** ✓ *(released)*

- [x] L7 HTTP reverse proxy
- [x] L4 TCP tunneling
- [x] Orbital Router — round-robin load balancer
- [x] Pulse Scan health checks with auto-recovery
- [x] Config hot-reload via `fsnotify`
- [x] Multi-stage Docker build
- [x] Graceful shutdown

**v0.2.0 — Engaging Engines** ✓ *(released)*

- [x] Config schema extended — TLS, FluxShield, balancer algorithm, metrics fields
- [x] Orbital Router — least-connections + weighted round-robin algorithms
- [x] Flux Shield — per-IP token-bucket rate limiting (429 on excess)
- [x] Stellar Encryption — HTTPS with Let's Encrypt auto-cert or manual cert/key
- [x] Prometheus metrics endpoint + structured JSON request logging (Stellar Log)
- [x] SQLite metrics persistence (historical data for dashboard)
- [x] UDP tunneling — L4 datagram forwarding with sticky sessions

**v0.3.0 — Quantum Link Established** *(in progress)*

- [x] Config schema extended — `oracle:` block with env-var API key expansion
- [x] SQLite `threat_signals` table — persistent Oracle alert storage with SaveSignal, ListSignals, ResolveSignal
- [x] Metrics aggregator — converts raw Prometheus counters into rate/error/latency snapshots with delta tracking
- [x] The Oracle core engine — Claude-powered threat detection and scaling advisor with threshold-triggered and interval-based analysis
- [x] Oracle signals HTTP endpoint — GET active alerts as JSON, POST to resolve by ID
- [x] Oracle wired into main — starts at boot, threshold checks every 30 s, signals at `/oracle/signals`
- [x] `centauri init` CLI wizard — interactive config generator, writes `centauri.yml` via guided prompts

**On the Horizon**

| Feature | Codename | Notes |
|---|---|---|
| Web dashboard | Mission Control | Visual gate management, live traffic metrics |
| WebSocket support | — | HTTP upgrade path through Jump Gates |

---

## Project Structure

```text
proxy-centauri/
├── cmd/
│   └── centauri/
│       └── main.go            # Entry point — boots all subsystems, ASCII logo
├── internal/
│   ├── balancer/
│   │   ├── balancer.go        # Balancer interface + NewFromConfig factory
│   │   ├── roundrobin.go      # Orbital Router — atomic round-robin LB
│   │   ├── leastconn.go       # Least-connections LB with Acquire/Release
│   │   └── weighted.go        # Nginx smooth weighted round-robin
│   ├── metrics/
│   │   ├── collector.go       # Prometheus metric vars + Init() + Handler() + Snapshots()
│   │   ├── middleware.go      # HTTP instrumentation middleware
│   │   └── store.go           # SQLite persistence — request_stats + events tables
│   ├── logger/
│   │   └── stellar.go         # Stellar Log — structured JSON request logger
│   ├── ratelimit/
│   │   ├── fluxshield.go      # Flux Shield — per-IP token-bucket rate limiter
│   │   └── fluxshield_test.go
│   ├── tls/
│   │   ├── stellar.go         # Stellar Encryption — AutoCert (Let's Encrypt) + ManualCert
│   │   └── stellar_test.go
│   ├── config/
│   │   ├── config.go          # YAML structs + Load()
│   │   └── watcher.go         # Hot-reload via fsnotify
│   ├── health/
│   │   └── pulsescan.go       # Pulse Scan — HTTP + TCP health checks
│   ├── proxy/
│   │   └── proxy.go           # L7 HTTP reverse proxy
│   └── tunnel/
│       ├── tunnel.go          # L4 TCP tunnel
│       └── udp.go             # L4 UDP tunnel — sticky sessions, idle eviction
├── logs/                      # Per-gate JSON request logs (auto-created)
├── docs/
│   └── PROGRESS.md
├── centauri.example.yml       # Annotated config template
├── centauri.yml               # Active config (do not commit secrets)
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Contributing

Contributions are welcome. Please open an issue before submitting a pull request for anything beyond typo fixes. The project is early-stage (v0.3.0) — check the roadmap for planned work before starting something major.

```bash
git clone https://github.com/eliot-lemaire/proxy-centauri.git
cd proxy-centauri
go test ./...
```

---

## License

MIT License

Copyright (c) 2026 Eliot Lemaire

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
