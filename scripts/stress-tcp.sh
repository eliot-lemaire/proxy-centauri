#!/usr/bin/env bash
# scripts/stress-tcp.sh
# Opens N concurrent TCP connections through the tunnel and verifies each echo.
# Reports success rate and any failures.
#
# Configurable via env vars:
#   STRESS_TCP_CONNS   — parallel connections  (default: 50)
#   TCP_HOST / TCP_PORT — target (default: localhost:9000)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TCP_HOST="${TCP_HOST:-localhost}"
TCP_PORT="${TCP_PORT:-9000}"
CONNECTIONS="${STRESS_TCP_CONNS:-50}"
RESULTS_DIR=$(mktemp -d)
trap 'rm -rf "$RESULTS_DIR"' EXIT

header "TCP Stress Test"
info "Target:      $TCP_HOST:$TCP_PORT"
info "Connections: $CONNECTIONS parallel"

wait_for_port "$TCP_HOST" "$TCP_PORT" 30 || {
    fail "TCP gate :$TCP_PORT not reachable after 30s"
    summary
}

do_tcp() {
    local id="$1"
    local payload="stress-tcp-$id"
    local response
    response=$(printf '%s' "$payload" | nc -w 4 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
    if [[ "$response" == "$payload" ]]; then
        echo "ok" > "$RESULTS_DIR/$id"
    else
        echo "fail" > "$RESULTS_DIR/$id"
    fi
}

start_ns=$(date +%s%N)

pids=()
for i in $(seq 1 "$CONNECTIONS"); do
    do_tcp "$i" &
    pids+=($!)
done
for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done

end_ns=$(date +%s%N)
elapsed_ms=$(( (end_ns - start_ns) / 1000000 ))

ok=0; fail_count=0
for f in "$RESULTS_DIR"/*; do
    [[ "$(cat "$f")" == "ok" ]] && ok=$((ok + 1)) || fail_count=$((fail_count + 1))
done

success_pct=$(( ok * 100 / CONNECTIONS ))

info "─────────────────────────────────"
info "Duration:    ${elapsed_ms} ms"
info "Successful:  $ok / $CONNECTIONS ($success_pct%)"
info "Failed:      $fail_count / $CONNECTIONS"
info "─────────────────────────────────"

[[ $success_pct -ge 95 ]] \
    && pass "TCP stress: $ok / $CONNECTIONS connections succeeded ($success_pct%)" \
    || fail "TCP stress: only $ok / $CONNECTIONS succeeded ($success_pct%)"

summary
