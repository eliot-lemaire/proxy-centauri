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

         ✦  Navigating the cosmos, one request at a time  ✦
              v0.1.0 — Milestone 1: First Contact
```

# Proxy Centauri

*A lightweight, config-driven reverse proxy for HTTP and TCP — built in Go.*

[![Go 1.22](https://img.shields.io/badge/go-1.22-00ADD8?logo=go&logoColor=white&style=flat-square)](https://golang.org/doc/go1.22)
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
| **Orbital Router — Round-Robin LB** | Lock-free atomic counter; concurrent-safe; distributes requests evenly across all live Star Systems |
| **Pulse Scan Health Checker** | HTTP: `GET` with 2s timeout (any `< 500` response = alive); TCP: raw dial with 2s timeout; checks every 5s; auto-removes dead backends and restores them on recovery |
| **Config Hot-Reload** | `fsnotify` watches `centauri.yml`; reloads without dropping connections or restarting the process |
| **Multi-Stage Docker Build** | `golang:1.22-alpine` builder → `alpine:3.19` runtime; final image ~10 MB |
| **Graceful Shutdown** | Listens for `SIGINT`/`SIGTERM`; drains cleanly before exit |

---

## Architecture

```text
  Incoming Traffic
        │
        ├── HTTP :8000 ──▶ ┌──────────────────────────┐
        │                  │     JUMP GATE (HTTP)      │
        └── TCP  :9000 ──▶ │     JUMP GATE (TCP)       │
                           └────────────┬─────────────┘
                                        │
                             ┌──────────▼──────────┐
                             │    ORBITAL ROUTER   │
                             │   (Round-Robin LB)  │
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
| `jump_gates[].protocol` | string | yes | Transport protocol: `http` or `tcp` |
| `jump_gates[].star_systems[].address` | string | yes | Backend `host:port` — supports DNS names and IPs |
| `jump_gates[].star_systems[].weight` | int | no | Reserved for weighted LB — parsed but not yet used |

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

**v0.1.0 — First Contact** ✓

- [x] L7 HTTP reverse proxy
- [x] L4 TCP tunneling
- [x] Orbital Router — round-robin load balancer
- [x] Pulse Scan health checks with auto-recovery
- [x] Config hot-reload via `fsnotify`
- [x] Multi-stage Docker build
- [x] Graceful shutdown

**On the Horizon**

| Feature | Codename | Notes |
|---|---|---|
| Web dashboard | Mission Control | Visual gate management, live traffic metrics |
| AI routing engine | The Oracle | Claude / OpenAI-driven routing decisions and anomaly detection |
| HTTPS + auto-TLS | — | ACME / Let's Encrypt integration (`tls: auto` in config) |
| Metrics export | — | Prometheus `/metrics` endpoint |
| Weighted load balancing | Orbital Router v2 | Activates the `weight` field already parsed in config |
| WebSocket support | — | HTTP upgrade path through Jump Gates |
| UDP tunneling | — | L4 extension alongside TCP |

---

## Project Structure

```text
proxy-centauri/
├── cmd/
│   └── centauri/
│       └── main.go            # Entry point — boots all subsystems, ASCII logo
├── internal/
│   ├── balancer/
│   │   ├── roundrobin.go      # Orbital Router — atomic round-robin LB
│   │   └── roundrobin_test.go
│   ├── config/
│   │   ├── config.go          # YAML structs + Load()
│   │   └── watcher.go         # Hot-reload via fsnotify
│   ├── health/
│   │   └── pulsescan.go       # Pulse Scan — HTTP + TCP health checks
│   ├── proxy/
│   │   └── proxy.go           # L7 HTTP reverse proxy
│   └── tunnel/
│       └── tunnel.go          # L4 TCP tunnel
├── docs/
│   ├── plan.md
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

Contributions are welcome. Please open an issue before submitting a pull request for anything beyond typo fixes. The project is early-stage (v0.1.0) — check the roadmap for planned work before starting something major.

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
