#!/usr/bin/env bash
# scripts/test-healthcheck.sh
# Tests Pulse Scan health checking by stopping a backend and verifying the proxy
# detects the failure (503) and recovers when the backend restarts (200).
#
# Uses the single-backend base config (centauri.example.yml / docker-compose.yml).
# Run BEFORE the test overlay, or set COMPOSE_FILES accordingly.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

COMPOSE_FILES="${COMPOSE_FILES:--f docker-compose.yml}"
COMPOSE_CMD="docker compose $COMPOSE_FILES"
HTTP_BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"

header "Health Check & Recovery Test (Pulse Scan)"
info "Using compose: $COMPOSE_CMD"

wait_for_http "$HTTP_BASE/" 30 || { fail "HTTP gate not ready"; summary; }

# Baseline
code=$(curl -s -o /dev/null -w "%{http_code}" "$HTTP_BASE/" 2>/dev/null)
[[ "$code" == "200" ]] \
    && pass "Baseline: HTTP gate → 200 OK" \
    || fail "Baseline: got $code (expected 200)"

# ── Stop all HTTP backends ─────────────────────────────────────────────────────
# Detect how many backends are running (base config has 1, test overlay has 3)
all_http_backends="echo-http"
for svc in echo-http-2 echo-http-3; do
    if $COMPOSE_CMD ps "$svc" 2>/dev/null | grep -q "Up\|running"; then
        all_http_backends="$all_http_backends $svc"
    fi
done
info "Stopping HTTP backends: $all_http_backends"
# shellcheck disable=SC2086
$COMPOSE_CMD stop $all_http_backends 2>/dev/null || {
    fail "Could not stop HTTP backends — is Docker running?"
    # shellcheck disable=SC2086
    $COMPOSE_CMD start $all_http_backends 2>/dev/null || true
    summary
}

# Wait for PulseScan to detect failure (health checks run every 5s, allow 3 cycles)
info "Waiting up to 20s for Pulse Scan to mark backend dead..."
failure_detected=false
for i in $(seq 1 20); do
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 "$HTTP_BASE/" 2>/dev/null || echo "000")
    if [[ "$code" == "503" ]]; then
        failure_detected=true
        info "Failure detected after ${i}s"
        break
    fi
    sleep 1
done

if $failure_detected; then
    pass "Backend failure detected: gate returns 503 when no backends healthy"
else
    fail "Backend failure not detected after 20s (expected 503, last code=$code)"
fi

# ── Restart all HTTP backends ──────────────────────────────────────────────────
info "Restarting HTTP backends: $all_http_backends"
# shellcheck disable=SC2086
$COMPOSE_CMD start $all_http_backends 2>/dev/null || {
    fail "Could not restart HTTP backends"
    summary
}

# Wait for recovery (backend boot + health check cycle)
info "Waiting up to 30s for recovery..."
recovered=false
for i in $(seq 1 30); do
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 "$HTTP_BASE/" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        recovered=true
        info "Recovery confirmed after ${i}s"
        break
    fi
    sleep 1
done

if $recovered; then
    pass "Backend recovery: gate returns 200 after echo-http restart"
else
    fail "Backend did not recover within 30s (last code=$code)"
fi

summary
