#!/usr/bin/env bash
# scripts/stress-http.sh
# HTTP stress test using parallel curl background jobs.
# No external tools required — pure bash + curl.
#
# Configurable via env vars:
#   STRESS_CONCURRENCY  — parallel workers          (default: 20)
#   STRESS_TOTAL        — total requests to send    (default: 300)
#   HTTP_HOST / HTTP_PORT — target (default: localhost:8000)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"
CONCURRENCY="${STRESS_CONCURRENCY:-20}"
TOTAL="${STRESS_TOTAL:-300}"
RESULTS_DIR=$(mktemp -d)
trap 'rm -rf "$RESULTS_DIR"' EXIT

header "HTTP Stress Test"
info "Target:      $BASE"
info "Concurrency: $CONCURRENCY workers"
info "Total:       $TOTAL requests"

wait_for_http "$BASE/" 30 || { fail "HTTP gate not ready"; summary; }

# Worker: fires one request and writes the status code to a temp file.
# Uses a unique X-Forwarded-For per request so per-IP rate limiting does not
# artificially reduce throughput during the stress run.
do_request() {
    local id="$1"
    curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        -H "X-Forwarded-For: 10.$((id % 256)).$((id / 256 % 256)).1" \
        "$BASE/" 2>/dev/null \
        > "$RESULTS_DIR/$id" || echo "000" > "$RESULTS_DIR/$id"
}

start_ns=$(date +%s%N)

completed=0
while [[ $completed -lt $TOTAL ]]; do
    batch=$(( CONCURRENCY < (TOTAL - completed) ? CONCURRENCY : (TOTAL - completed) ))
    pids=()
    for i in $(seq 1 "$batch"); do
        do_request "$((completed + i))" &
        pids+=($!)
    done
    for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
    completed=$((completed + batch))
    printf "\r  [INFO] Progress: %d / %d" "$completed" "$TOTAL"
done
printf "\n"

end_ns=$(date +%s%N)
elapsed_ms=$(( (end_ns - start_ns) / 1000000 ))
elapsed_s=$(( elapsed_ms / 1000 ))
[[ $elapsed_s -eq 0 ]] && elapsed_s=1

# Tally
c2xx=0; c4xx=0; c5xx=0; cerr=0
for f in "$RESULTS_DIR"/*; do
    code=$(cat "$f")
    case "$code" in
        2??) c2xx=$((c2xx + 1)) ;;
        4??) c4xx=$((c4xx + 1)) ;;
        5??) c5xx=$((c5xx + 1)) ;;
        *)   cerr=$((cerr + 1)) ;;
    esac
done

rps=$(( TOTAL / elapsed_s ))
success_pct=$(( c2xx * 100 / TOTAL ))
error_pct=$(( (c5xx + cerr) * 100 / TOTAL ))

info "─────────────────────────────────"
info "Duration:        ${elapsed_ms} ms"
info "Throughput:      ~${rps} req/s"
info "2xx success:     $c2xx / $TOTAL ($success_pct%)"
info "4xx client err:  $c4xx / $TOTAL"
info "5xx server err:  $c5xx / $TOTAL"
info "Timeout/error:   $cerr / $TOTAL"
info "─────────────────────────────────"

[[ $success_pct -ge 95 ]] \
    && pass "Success rate ≥ 95%: ${success_pct}% ($c2xx / $TOTAL)" \
    || fail "Success rate below 95%: ${success_pct}% ($c2xx / $TOTAL)"

[[ $error_pct -le 5 ]] \
    && pass "Error rate ≤ 5%: ${error_pct}% ($((c5xx + cerr)) / $TOTAL)" \
    || fail "Error rate above 5%: ${error_pct}%"

[[ $rps -ge 10 ]] \
    && pass "Throughput ≥ 10 req/s: ~${rps} req/s" \
    || warn "Throughput below 10 req/s: ~${rps} req/s (normal for Docker on resource-limited hosts)"

summary
