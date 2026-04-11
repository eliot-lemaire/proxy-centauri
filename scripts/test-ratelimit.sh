#!/usr/bin/env bash
# scripts/test-ratelimit.sh
# Verifies Flux Shield rate limiting by hammering a burst of requests rapidly.
# centauri.test.yml sets requests_per_second=5, burst=10.
# Fires 30 rapid requests; after the burst bucket empties some should be 429.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"
BURST="${RATE_BURST:-10}"
TOTAL="${RATE_TOTAL:-30}"

header "Rate Limiting (Flux Shield) Test"
info "Firing $TOTAL rapid requests — burst=$BURST, rps=5"
info "Expected: first ~$BURST → 200, remainder → 429"

wait_for_http "$BASE/" 30 || { fail "HTTP gate not ready after 30s"; summary; }

count_200=0
count_429=0
count_other=0

# Pin all requests to the same fake source IP via X-Forwarded-For
# so they all hit the same per-IP token bucket.
for i in $(seq 1 "$TOTAL"); do
    code=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "X-Forwarded-For: 10.99.99.99" \
        --max-time 2 \
        "$BASE/" 2>/dev/null || echo "000")
    case "$code" in
        200) count_200=$((count_200 + 1)) ;;
        429) count_429=$((count_429 + 1)) ;;
        *)   count_other=$((count_other + 1)) ;;
    esac
done

info "200 OK:                $count_200"
info "429 Too Many Requests: $count_429"
[[ $count_other -gt 0 ]] && warn "Other/timeout codes:   $count_other"

# Rate limiting must have triggered at least once
[[ $count_429 -gt 0 ]] \
    && pass "Rate limit triggered: $count_429 × 429 received" \
    || fail "Rate limit never triggered — got 0 × 429 out of $TOTAL requests"

# Burst should have been honored (at least BURST requests succeeded)
[[ $count_200 -ge "$BURST" ]] \
    && pass "Burst honored: $count_200 requests succeeded (≥ burst of $BURST)" \
    || fail "Too few successes: $count_200 (expected at least burst of $BURST)"

# Verify Retry-After header on the next 429
info "Checking Retry-After header on a rate-limited response..."
response_headers=$(curl -s -D - -o /dev/null \
    -H "X-Forwarded-For: 10.99.99.99" \
    --max-time 2 \
    "$BASE/" 2>/dev/null || true)
if echo "$response_headers" | grep -qi "retry-after: 1"; then
    pass "429 response includes Retry-After: 1 header"
else
    warn "Retry-After header not found (bucket may have partially refilled between checks)"
fi

summary
