# Proxy Centauri — Build Log

---

## 2026-04-04 — Milestone 1, Step 1: Go project scaffold + ASCII logo

**What was built:**
- Initialized Go module: `github.com/eliot-lemaire/proxy-centauri` (`go.mod`)
- Created `cmd/centauri/main.go` — the program entry point
- ASCII logo prints on startup ("PROXY CENTAURI" in block letters)
- Mission Control status lines print below the logo
- Created `internal/` directory structure for future packages: `config/`, `proxy/`, `tunnel/`, `health/`, `balancer/`

**How to run:**
```bash
~/go/bin/go run ./cmd/centauri
```

**Status:** Logo prints, program runs, no crashes. Step 1 of Milestone 1 complete.
