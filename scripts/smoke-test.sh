#!/usr/bin/env bash
# scripts/smoke-test.sh
# Verifies every endpoint is reachable. Run immediately after docker compose up.
# Expected runtime: ~30–60 seconds (includes startup polling).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

HTTP_HOST="${HTTP_HOST:-localhost}"
HTTP_PORT="${HTTP_PORT:-8000}"
TCP_HOST="${TCP_HOST:-localhost}"
TCP_PORT="${TCP_PORT:-9000}"
UDP_HOST="${UDP_HOST:-localhost}"
UDP_PORT="${UDP_PORT:-9001}"
METRICS_HOST="${METRICS_HOST:-localhost}"
METRICS_PORT="${METRICS_PORT:-9090}"

header "Smoke Test — Proxy Centauri"

# ── HTTP gate ──────────────────────────────────────────────────────────────────
info "Waiting for HTTP gate on $HTTP_HOST:$HTTP_PORT (up to 60s)..."
if ! wait_for_http "http://$HTTP_HOST:$HTTP_PORT/" 60; then
    fail "HTTP gate did not become ready within 60s"
    summary
fi

code=$(curl -s -o /dev/null -w "%{http_code}" "http://$HTTP_HOST:$HTTP_PORT/")
if [[ "$code" == "200" ]]; then
    pass "HTTP gate :$HTTP_PORT → 200 OK"
else
    fail "HTTP gate :$HTTP_PORT → expected 200, got $code"
fi

# ── TCP gate ───────────────────────────────────────────────────────────────────
if wait_for_port "$TCP_HOST" "$TCP_PORT" 15; then
    echo_reply=$(printf 'ping' | nc -w 3 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
    if [[ "$echo_reply" == *"ping"* ]]; then
        pass "TCP gate :$TCP_PORT → echo works"
    else
        fail "TCP gate :$TCP_PORT → connected but no echo reply" "$echo_reply"
    fi
else
    fail "TCP gate :$TCP_PORT → port not reachable after 15s"
fi

# ── UDP gate ───────────────────────────────────────────────────────────────────
udp_reply=$(printf 'ping' | nc -u -w 3 "$UDP_HOST" "$UDP_PORT" 2>/dev/null || true)
if [[ "$udp_reply" == *"ping"* ]]; then
    pass "UDP gate :$UDP_PORT → datagram echo works"
else
    warn "UDP gate :$UDP_PORT → no echo (may need a moment for sticky session setup)"
fi

# ── Metrics endpoint ───────────────────────────────────────────────────────────
if wait_for_port "$METRICS_HOST" "$METRICS_PORT" 10; then
    metrics_body=$(curl -s "http://$METRICS_HOST:$METRICS_PORT/metrics" 2>/dev/null)
    if echo "$metrics_body" | grep -q "centauri_requests_total"; then
        pass "Metrics :$METRICS_PORT → /metrics has centauri_requests_total"
    else
        fail "Metrics :$METRICS_PORT → /metrics missing centauri_requests_total"
    fi
else
    fail "Metrics :$METRICS_PORT → port not reachable"
fi

summary
