# Proxy Centauri — Setup & Feature Guide

**v0.2.0 — Milestone 2: Engaging Engines**

A lightweight, config-driven reverse proxy for HTTP, TCP, and UDP traffic with load balancing, health checking, rate limiting, TLS, and Prometheus metrics.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Quick Start with Docker](#2-quick-start-with-docker)
3. [Configuration Reference](#3-configuration-reference)
4. [Features](#4-features)
   - 4.1 [HTTP Reverse Proxy](#41-http-reverse-proxy)
   - 4.2 [TCP Tunneling](#42-tcp-tunneling)
   - 4.3 [UDP Tunneling](#43-udp-tunneling)
   - 4.4 [Orbital Router — Load Balancing](#44-orbital-router--load-balancing)
   - 4.5 [Pulse Scan — Health Checking](#45-pulse-scan--health-checking)
   - 4.6 [Flux Shield — Rate Limiting](#46-flux-shield--rate-limiting)
   - 4.7 [Stellar Encryption — TLS/HTTPS](#47-stellar-encryption--tlshttps)
   - 4.8 [Config Hot-Reload](#48-config-hot-reload)
   - 4.9 [Stellar Log — JSON Request Logging](#49-stellar-log--json-request-logging)
5. [Monitoring](#5-monitoring)
   - 5.1 [Prometheus Metrics](#51-prometheus-metrics)
   - 5.2 [SQLite Metrics Persistence](#52-sqlite-metrics-persistence)
6. [TLS Setup](#6-tls-setup)
7. [Running the Test Suite](#7-running-the-test-suite)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. Prerequisites

| Requirement | Minimum | Notes |
|-------------|---------|-------|
| Docker | 24.x | For the recommended Docker workflow |
| Docker Compose | v2.x | `docker compose` (not `docker-compose`) |
| Go | 1.22+ | Only needed if building from source |

---

## 2. Quick Start with Docker

```bash
# Clone the repo
git clone https://github.com/eliot-lemaire/proxy-centauri.git
cd proxy-centauri

# Start the full stack (centauri + echo backends)
docker compose up --build -d

# Verify everything is running
docker compose ps
```

**Exposed ports by default:**

| Port | Purpose |
|------|---------|
| `8000` | HTTP reverse proxy |
| `9000` | TCP tunnel |
| `9001/udp` | UDP tunnel |
| `9090` | Prometheus metrics |

```bash
# Quick test
curl http://localhost:8000/          # HTTP proxy
echo "ping" | nc localhost 9000     # TCP tunnel
echo "ping" | nc -u localhost 9001  # UDP tunnel
curl http://localhost:9090/metrics  # Prometheus
```

### Customising the Config

The active config is mounted into the container. Edit `centauri.example.yml` (used by Docker) or create your own and update the volume mount in `docker-compose.yml`:

```yaml
volumes:
  - ./my-config.yml:/app/centauri.yml
```

Centauri watches for file changes and reloads automatically — no restart needed when updating `star_systems`.

---

## 3. Configuration Reference

```yaml
mission_control:
  port: 8080          # Reserved for future web dashboard
  secret: "change-me" # Reserved for future dashboard auth

jump_gates:
  - name: "my-gate"          # Unique identifier shown in logs
    listen: ":8000"           # Bind address (port or host:port)
    protocol: http            # http | tcp | udp
    orbital_router: round_robin  # See Load Balancing section
    tls:
      mode: ""                # "" (off) | "auto" (Let's Encrypt) | "manual"
      domain: ""              # Required when mode is "auto"
      cert_file: ""           # Required when mode is "manual"
      key_file: ""            # Required when mode is "manual"
    flux_shield:
      requests_per_second: 0  # 0 = disabled. Tokens refilled at this rate per IP
      burst: 0                # Max tokens above the steady-state rate
    star_systems:
      - address: "backend:3000"  # host:port of backend
        weight: 1                # Only used by the "weighted" router

metrics:
  enabled: true   # Expose /metrics on the Prometheus port
  port: 9090
```

### Full Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `jump_gates[].name` | string | — | Unique gate identifier. Used in logs, metrics labels, and log filenames. |
| `jump_gates[].listen` | string | — | Bind address e.g. `:8000` or `0.0.0.0:8000` |
| `jump_gates[].protocol` | string | — | `http`, `tcp`, or `udp` |
| `jump_gates[].orbital_router` | string | `round_robin` | Load balancing algorithm. See §4.4. |
| `jump_gates[].tls.mode` | string | `""` | TLS mode. `""` = plain, `"auto"` = Let's Encrypt, `"manual"` = custom cert. |
| `jump_gates[].tls.domain` | string | — | FQDN for Let's Encrypt. Required when `mode: "auto"`. |
| `jump_gates[].tls.cert_file` | string | — | Path to PEM cert. Required when `mode: "manual"`. |
| `jump_gates[].tls.key_file` | string | — | Path to PEM key. Required when `mode: "manual"`. |
| `jump_gates[].flux_shield.requests_per_second` | float | `0` | Token refill rate per source IP. `0` disables rate limiting. |
| `jump_gates[].flux_shield.burst` | int | `0` | Maximum burst above the steady rate. |
| `jump_gates[].star_systems[].address` | string | — | Backend `host:port`. |
| `jump_gates[].star_systems[].weight` | int | `0` | Weight for `weighted` algorithm. `0` is treated as `1`. |
| `metrics.enabled` | bool | `false` | Enable Prometheus scrape endpoint. |
| `metrics.port` | int | `9090` | Port for `/metrics`. |

---

## 4. Features

### 4.1 HTTP Reverse Proxy

Centauri proxies HTTP/1.1 traffic using Go's `httputil.ReverseProxy`. Every request is forwarded to the next healthy backend selected by the Orbital Router.

**What it does:**
- Preserves the original request method, path, query string, and headers
- Sets `X-Forwarded-For` to the real client IP
- Returns `503 Service Unavailable` when all backends are down
- Returns `502 Bad Gateway` on a backend connection error

**Example config:**
```yaml
- name: "web-app"
  listen: ":8000"
  protocol: http
  star_systems:
    - address: "app-server-1:3000"
    - address: "app-server-2:3000"
```

---

### 4.2 TCP Tunneling

Proxies raw TCP connections to one or more backends. Protocol-agnostic — works for MySQL, PostgreSQL, Redis, SSH, game servers, MQTT, and anything else over TCP.

**What it does:**
- Bidirectional byte piping with no protocol interpretation
- Each new TCP connection is routed by the Orbital Router
- No TLS at the L4 layer (use TLS at the application layer)

**Example config:**
```yaml
- name: "db-proxy"
  listen: ":5432"
  protocol: tcp
  orbital_router: least_connections
  star_systems:
    - address: "postgres-primary:5432"
    - address: "postgres-replica:5432"
```

---

### 4.3 UDP Tunneling

Proxies UDP datagrams with per-client sticky sessions. Each unique source address gets its own dedicated backend connection that persists for 30 seconds of idle time.

**What it does:**
- Per-client sticky sessions using a `sync.Map` keyed by source address
- Idle sessions are automatically evicted after 30 seconds
- Reply pump routes backend responses back to the original client
- Works for DNS, VoIP (SIP/RTP), game servers, QUIC, NTP, IoT sensors

**Example config:**
```yaml
- name: "game-udp"
  listen: ":7777"
  protocol: udp
  star_systems:
    - address: "game-server-1:7777"
    - address: "game-server-2:7777"
```

**Note:** UDP health checking sends a `"ping"` datagram and waits for any reply. If your backend doesn't reply to pings, health checks will mark it dead. Use a protocol-specific health check or set the backend to echo arbitrary datagrams during health probes.

---

### 4.4 Orbital Router — Load Balancing

Configured per gate with `orbital_router`. Three algorithms are available:

#### `round_robin` (default)
Distributes requests evenly across all healthy backends in strict rotation.
```
Request 1 → backend-A
Request 2 → backend-B
Request 3 → backend-C
Request 4 → backend-A  (wraps)
```
Uses an atomic counter — no mutex on the hot path.

#### `least_connections`
Routes each request to the backend with the fewest in-flight requests. Best for backends with variable response times.
```yaml
orbital_router: least_connections
```

#### `weighted`
Distributes traffic proportionally by weight. Use when backends have different capacities.
```yaml
orbital_router: weighted
star_systems:
  - address: "big-server:3000"
    weight: 3    # gets ~50% of traffic
  - address: "medium-server:3000"
    weight: 2    # gets ~33% of traffic
  - address: "small-server:3000"
    weight: 1    # gets ~17% of traffic
```
Uses the Nginx smooth weighted round-robin algorithm — weight 0 is treated as weight 1.

---

### 4.5 Pulse Scan — Health Checking

Centauri automatically monitors all backends every 5 seconds. Dead backends are removed from rotation; recovered backends are automatically restored.

**Check methods by protocol:**

| Protocol | Method | Pass condition |
|----------|--------|---------------|
| `http` | `GET /` with 2s timeout | Any status code < 500 |
| `tcp` | `net.DialTimeout` with 2s timeout | Connection succeeds |
| `udp` | Send `"ping"`, read reply with 2s timeout | Any bytes received back |

**What you see in logs:**
```
[ Pulse Scan ] web-app  backend:3000  is dead — removed from rotation
[ Pulse Scan ] web-app  backend:3000  recovered — back in rotation
```

**Behaviour when all backends are dead:** The HTTP gate returns `503 Service Unavailable`. The TCP tunnel rejects new connections. The UDP tunnel drops datagrams.

Health check events (`backend_up`, `backend_down`) are also recorded in the SQLite metrics store.

---

### 4.6 Flux Shield — Rate Limiting

Per-IP token-bucket rate limiting for HTTP gates. Each source IP gets its own independent bucket.

**How it works:**
- `requests_per_second` — tokens refilled at this rate continuously
- `burst` — the maximum number of tokens that can accumulate (handles short bursts)
- Excess requests get `429 Too Many Requests` with `Retry-After: 1`
- Idle buckets are evicted after 60 seconds to prevent memory growth
- Source IP extracted from `X-Forwarded-For` (leftmost value) with fallback to `RemoteAddr`

**Example — 100 requests/second with burst of 20:**
```yaml
flux_shield:
  requests_per_second: 100
  burst: 20
```

**Disable rate limiting** (zero overhead):
```yaml
flux_shield:
  requests_per_second: 0
  burst: 0
```

---

### 4.7 Stellar Encryption — TLS/HTTPS

Two TLS modes are available for HTTP gates.

#### Auto (Let's Encrypt)
Centauri obtains and renews certificates automatically via ACME HTTP-01 challenge. Requires a publicly routable domain and port 80 accessible for the challenge.

```yaml
tls:
  mode: "auto"
  domain: "proxy.example.com"
```

Certificates are cached in the `.certs/` directory next to the binary.

#### Manual (Custom Certificates)
Provide your own cert/key pair — self-signed, CA-issued, or from any source.

```bash
# Generate a self-signed cert for local testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj '/CN=localhost'
```

```yaml
tls:
  mode: "manual"
  cert_file: "/app/certs/cert.pem"
  key_file: "/app/certs/key.pem"
```

Mount your cert files into the Docker container:
```yaml
volumes:
  - ./certs:/app/certs:ro
```

---

### 4.8 Config Hot-Reload

Centauri watches `centauri.yml` for changes using `fsnotify`. When the file is saved:

1. The new config is parsed and validated
2. Running `star_systems` are updated immediately — backends added/removed take effect without a restart
3. Health checkers begin watching new addresses on their next tick
4. A `config_reload` event is recorded in the SQLite metrics store

**What requires a restart:**
- Adding or removing entire `jump_gates`
- Changing `listen`, `protocol`, or `orbital_router` on an existing gate
- Enabling/disabling metrics
- TLS mode changes

**What takes effect live (no restart):**
- Adding or removing backends in `star_systems`
- Changing `flux_shield` settings (takes effect on the next config reload parse; the running middleware is not swapped live in v0.2.0 — restart to change rate limit values)

---

### 4.9 Stellar Log — JSON Request Logging

Every HTTP request is logged to a per-gate JSON file in the `logs/` directory.

**Log location:** `logs/<gate-name>.log`

**Log format (one JSON object per line):**
```json
{
  "time": "2026-04-11T14:23:01Z",
  "gate": "web-app",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "latency_ms": 12,
  "client_ip": "203.0.113.42"
}
```

**Tail logs in Docker:**
```bash
docker exec goproxy-centauri-1 tail -f /app/logs/web-app.log
```

---

## 5. Monitoring

### 5.1 Prometheus Metrics

When `metrics.enabled: true`, Centauri exposes a Prometheus scrape endpoint at `http://host:9090/metrics`.

**Metrics exposed:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `centauri_requests_total` | Counter | `gate`, `status_code` | Total proxied HTTP requests |
| `centauri_request_duration_seconds` | Histogram | `gate` | Request latency in seconds |
| `centauri_active_connections` | Gauge | `gate` | In-flight HTTP requests |
| `centauri_errors_total` | Counter | `gate`, `error_type` | Proxy errors |

**Prometheus scrape config:**
```yaml
scrape_configs:
  - job_name: centauri
    static_configs:
      - targets: ['localhost:9090']
```

**Check metrics manually:**
```bash
curl http://localhost:9090/metrics | grep centauri_requests_total
```

### 5.2 SQLite Metrics Persistence

Centauri flushes a metrics snapshot every 30 seconds to `data/metrics.db`. This persists across restarts and can be queried with any SQLite client.

**Tables:**

```sql
-- Historical per-gate snapshots
SELECT * FROM request_stats ORDER BY ts DESC LIMIT 10;

-- Backend health events and config reloads
SELECT * FROM events ORDER BY ts DESC LIMIT 20;
```

**Access from Docker:**
```bash
docker exec goproxy-centauri-1 sh -c \
  "sqlite3 /app/data/metrics.db 'SELECT * FROM request_stats ORDER BY ts DESC LIMIT 5;'"
```

**Columns:**

`request_stats`: `ts` (unix timestamp), `gate`, `req_total`, `err_total`, `p95_ms`

`events`: `ts`, `gate`, `kind` (`backend_up` / `backend_down` / `config_reload`), `detail` (address or filename)

---

## 6. TLS Setup

### Auto — Let's Encrypt

Requirements:
- A public domain name with DNS pointing to your server
- Port 80 must be reachable for the ACME HTTP-01 challenge
- The process must have write access to the `.certs/` directory

```yaml
jump_gates:
  - name: "secure-app"
    listen: ":443"
    protocol: http
    tls:
      mode: "auto"
      domain: "proxy.example.com"
    star_systems:
      - address: "backend:3000"
```

Centauri starts an HTTP listener on port 80 automatically for the challenge. Certificates are renewed before expiry.

### Manual — Self-Signed (Local Testing)

```bash
# Generate self-signed cert
openssl req -x509 -newkey rsa:4096 -keyout certs/key.pem -out certs/cert.pem \
  -days 365 -nodes -subj '/CN=localhost'
```

```yaml
# docker-compose.yml — mount certs
volumes:
  - ./certs:/app/certs:ro
```

```yaml
# centauri.yml
jump_gates:
  - name: "local-https"
    listen: ":8443"
    protocol: http
    tls:
      mode: "manual"
      cert_file: "/app/certs/cert.pem"
      key_file: "/app/certs/key.pem"
    star_systems:
      - address: "backend:3000"
```

```bash
# Test with curl (skip cert verification for self-signed)
curl -k https://localhost:8443/
```

---

## 7. Running the Test Suite

All test scripts are in `scripts/`. They require the Docker stack to be running.

### Setup

```bash
# Start the full test stack (3 HTTP backends + TCP + UDP)
docker compose -f docker-compose.yml -f docker-compose.test.yml up --build -d

# Wait for all services to be healthy (check with)
docker compose -f docker-compose.yml -f docker-compose.test.yml ps
```

### Functional Tests

```bash
./scripts/smoke-test.sh          # Basic reachability (~30s)
./scripts/test-http.sh           # HTTP proxy behavior
./scripts/test-lb.sh             # Load balancer distribution
./scripts/test-ratelimit.sh      # Flux Shield rate limiting
./scripts/test-tcp.sh            # TCP tunnel echo
./scripts/test-udp.sh            # UDP tunnel echo
./scripts/test-metrics.sh        # Prometheus metrics
COMPOSE_FILES="-f docker-compose.yml -f docker-compose.test.yml" \
  ./scripts/test-healthcheck.sh  # Health check + recovery
COMPOSE_FILES="-f docker-compose.yml -f docker-compose.test.yml" \
  ./scripts/test-hotreload.sh    # Config hot-reload
```

### Stress Tests

```bash
./scripts/stress-http.sh         # HTTP concurrent load (default: 300 req, 20 workers)
./scripts/stress-tcp.sh          # TCP concurrent connections (default: 50)
./scripts/stress-udp.sh          # UDP datagram storm (default: 100 datagrams)
```

**Configurable via env vars:**
```bash
# HTTP stress: 500 requests with 50 concurrent workers
STRESS_TOTAL=500 STRESS_CONCURRENCY=50 ./scripts/stress-http.sh

# UDP delivery threshold (default 80%)
UDP_PASS_PCT=90 ./scripts/stress-udp.sh
```

### Unit Tests

```bash
go test ./... -race    # All packages, with race detector
```

### Tear Down

```bash
docker compose -f docker-compose.yml -f docker-compose.test.yml down -v
```

---

## 8. Troubleshooting

### HTTP gate returns 503

All backends are unhealthy. Pulse Scan has removed them from rotation.

```bash
# Check which backends are alive
docker compose logs centauri | grep "Pulse Scan"

# Check if your backend is actually reachable from inside the container
docker exec goproxy-centauri-1 wget -qO- http://your-backend:3000/
```

### HTTP gate returns 502

A backend was selected but the connection failed (backend crashed mid-request, or responded with a non-HTTP protocol on an HTTP gate).

### UDP datagrams not echoed

- Allow a few seconds after startup for the UDP sticky session to be established
- The socat UDP echo backend requires the health check to pass first
- Check container logs: `docker compose logs echo-udp`
- Verify the UDP port is not blocked by a firewall: `nc -u -w 2 localhost 9001`

### Rate limiter not triggering

- Verify `requests_per_second` and `burst` are non-zero in your config
- Confirm requests come from the same source IP — rate limiting is per-IP
- Use `X-Forwarded-For` to pin a test IP: `curl -H "X-Forwarded-For: 1.2.3.4" ...`

### Config reload not updating rate limit values

Rate limit values (`flux_shield`) are read at startup and baked into the running middleware. Changing them in the config requires a container restart in v0.2.0. Backend list changes (`star_systems`) do take effect immediately.

### Metrics counter stays at zero

The `/metrics` endpoint only shows counters that have been incremented. Send at least one HTTP request to the gate to initialise the counter:

```bash
curl http://localhost:8000/
curl http://localhost:9090/metrics | grep centauri_requests_total
```

All four metric families are pre-initialised at startup (even with zero traffic) so the HELP and TYPE lines always appear.

### Container fails to start

```bash
# Check centauri startup logs
docker compose logs centauri

# Common causes:
# - Port already in use → change listen: ":XXXX" in the config
# - Invalid YAML syntax → validate with: python3 -c "import yaml; yaml.safe_load(open('centauri.yml'))"
# - Backend address unreachable at startup → Pulse Scan will handle this; not a fatal error
```
