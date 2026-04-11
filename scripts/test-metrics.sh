#!/usr/bin/env bash
# scripts/test-metrics.sh
# Verifies the Prometheus /metrics endpoint exposes all expected metric families
# and that counters increment after real HTTP requests are sent.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

METRICS_BASE="http://${METRICS_HOST:-localhost}:${METRICS_PORT:-9090}"
HTTP_BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"

header "Prometheus Metrics Test"

wait_for_port "${METRICS_HOST:-localhost}" "${METRICS_PORT:-9090}" 30 || {
    fail "Metrics endpoint :${METRICS_PORT:-9090} not reachable after 30s"
    summary
}

# 1. Endpoint responds with 200
code=$(curl -s -o /dev/null -w "%{http_code}" "$METRICS_BASE/metrics")
[[ "$code" == "200" ]] && pass "/metrics → 200 OK" || fail "/metrics → got $code"

metrics_body=$(curl -s "$METRICS_BASE/metrics" 2>/dev/null)

# 2. All four metric families are present
for metric in \
    "centauri_requests_total" \
    "centauri_request_duration_seconds" \
    "centauri_active_connections" \
    "centauri_errors_total"; do
    if echo "$metrics_body" | grep -q "^# HELP $metric"; then
        pass "Metric family present: $metric"
    else
        fail "Metric family missing: $metric"
    fi
done

# 3. Generate traffic so counters increment
wait_for_http "$HTTP_BASE/" 30 || { fail "HTTP gate not ready"; summary; }
info "Sending 10 requests to generate metrics..."
for i in $(seq 1 10); do
    curl -s -o /dev/null "$HTTP_BASE/" 2>/dev/null || true
done

# Refresh metrics snapshot
metrics_body=$(curl -s "$METRICS_BASE/metrics" 2>/dev/null)

# 4. centauri_requests_total must have a non-zero counter
if echo "$metrics_body" | grep -qE 'centauri_requests_total\{[^}]+\} [1-9]'; then
    pass "centauri_requests_total has non-zero value after requests"
else
    fail "centauri_requests_total has no non-zero values after 10 requests"
fi

# 5. Labels are present
if echo "$metrics_body" | grep -q 'centauri_requests_total{gate='; then
    pass "centauri_requests_total has gate= label"
else
    fail "centauri_requests_total missing gate= label"
fi

if echo "$metrics_body" | grep -q 'status_code='; then
    pass "status_code= label present in request counter"
else
    fail "status_code= label missing from request counter"
fi

# 6. Histogram buckets are present
if echo "$metrics_body" | grep -q 'centauri_request_duration_seconds_bucket'; then
    pass "centauri_request_duration_seconds_bucket lines present"
else
    fail "centauri_request_duration_seconds_bucket missing"
fi

# 7. active_connections gauge is present
if echo "$metrics_body" | grep -qE 'centauri_active_connections\{'; then
    pass "centauri_active_connections gauge present"
else
    fail "centauri_active_connections gauge missing"
fi

summary
