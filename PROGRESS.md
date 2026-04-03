# Proxy Centauri — Build Progress

Tracks every feature and change added to the project. Updated automatically after each commit.

---

## Current Milestone
**Stage 2 — Project Setup** (pre-Milestone 1)

---

## Log

### 2026-04-03
- **Repo initialised** — Public GitHub repo created at `eliot-lemaire/proxy-centauri` with `main` branch
- **`.gitignore`** — Excludes binaries, `centauri.yml`, all `.env` / `.env.*` files, build artifacts, IDE folders
- **`.env.exemple`** — Sample environment variable template created (local only, not committed)
- **`PROGRESS.md`** — This file. Auto-documentation system in place

---

## Milestone Checklist

### Milestone 1 — The Core (Weeks 1–3) `[ not started ]`
- [ ] Go project scaffold + `cmd/centauri/main.go` + ASCII logo
- [ ] `centauri.yml` parser with hot-reload
- [ ] L7 HTTP reverse proxy (single upstream)
- [ ] L4 TCP tunnel (single upstream)
- [ ] Pulse Scan health check loop
- [ ] Round-robin load balancing
- [ ] Docker + Docker Compose setup

### Milestone 2 — Intelligence (Weeks 4–6) `[ not started ]`
- [ ] Least-connections + weighted load balancing
- [ ] Flux Shield rate limiting
- [ ] TLS — Let's Encrypt auto-cert + manual cert
- [ ] Prometheus `/metrics` endpoint
- [ ] Stellar Log (structured JSON logging)
- [ ] SQLite metrics persistence
- [ ] UDP tunnel support

### Milestone 3 — The Oracle (Weeks 7–8) `[ not started ]`
- [ ] Metrics aggregation pipeline
- [ ] Threat detection via Claude API
- [ ] Scaling advisor
- [ ] Oracle alert system + Threat Signals
- [ ] `centauri init` CLI wizard

### Milestone 4 — Mission Control (Weeks 9–11) `[ not started ]`
- [ ] REST API (`internal/api/`)
- [ ] JWT authentication
- [ ] React frontend (embedded via `go:embed`)
- [ ] First-run wizard (5-step UI flow)
- [ ] Dashboard: live traffic, Star System status
- [ ] Jump Gate manager
- [ ] Oracle panel
- [ ] Stellar Log viewer
- [ ] UI config writes back to `centauri.yml`

### Milestone 5 — Launch Ready (Week 12) `[ not started ]`
- [ ] Branding + logo in UI
- [ ] README quick-start guide
- [ ] Docker Hub publish
- [ ] Docs site
- [ ] Security audit
- [ ] Beta announcement
