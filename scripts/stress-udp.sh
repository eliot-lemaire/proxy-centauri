#!/usr/bin/env bash
# scripts/stress-udp.sh
# UDP datagram storm test — sends N datagrams and counts echo replies.
# UDP is inherently lossy; ≥80% delivery is the passing threshold.
#
# Configurable via env vars:
#   STRESS_UDP_DGRAMS  — number of datagrams   (default: 100)
#   UDP_HOST / UDP_PORT — target (default: localhost:9001)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

UDP_HOST="${UDP_HOST:-localhost}"
UDP_PORT="${UDP_PORT:-9001}"
DATAGRAMS="${STRESS_UDP_DGRAMS:-100}"
DELIVERY_THRESHOLD="${UDP_PASS_PCT:-80}"

header "UDP Stress Test (Datagram Storm)"
info "Target:    $UDP_HOST:$UDP_PORT"
info "Datagrams: $DATAGRAMS"
info "Pass threshold: ≥ ${DELIVERY_THRESHOLD}% delivery"

# Allow time for sticky session table to be ready
sleep 2

ok=0; lost=0

for i in $(seq 1 "$DATAGRAMS"); do
    payload="dgram-$i-$$"
    response=$(printf '%s' "$payload" | nc -u -w 2 "$UDP_HOST" "$UDP_PORT" 2>/dev/null || true)
    if [[ "$response" == "$payload" ]]; then
        ok=$((ok + 1))
    else
        lost=$((lost + 1))
    fi
done

delivery_pct=$(( ok * 100 / DATAGRAMS ))

info "─────────────────────────────────"
info "Delivered (echoed): $ok / $DATAGRAMS ($delivery_pct%)"
info "Lost / no-echo:     $lost / $DATAGRAMS"
info "─────────────────────────────────"

[[ $delivery_pct -ge $DELIVERY_THRESHOLD ]] \
    && pass "UDP delivery rate ≥ ${DELIVERY_THRESHOLD}%: ${delivery_pct}% ($ok / $DATAGRAMS)" \
    || fail "UDP delivery rate below ${DELIVERY_THRESHOLD}%: ${delivery_pct}% ($ok / $DATAGRAMS)"

summary
