#!/usr/bin/env bash
# scripts/test-lb.sh
# Tests load balancer distribution across multiple backends.
# Requires docker-compose.test.yml (3 HTTP backends on ports 3000/3001/3002).
#
# Detection: ealen/echo-server echoes back "PORT" env var in its JSON response.
# Each backend has a distinct port, so we can tell which one replied.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"
REQUESTS="${LB_REQUESTS:-60}"
TOLERANCE="${LB_TOLERANCE:-7}"

header "Load Balancer Distribution Test (Round-Robin)"
info "Sending $REQUESTS requests to $BASE — expecting even distribution across 3 backends"

wait_for_http "$BASE/" 30 || { fail "HTTP gate not ready after 30s"; summary; }

count_3000=0
count_3001=0
count_3002=0
count_other=0

for i in $(seq 1 "$REQUESTS"); do
    # Use a unique source IP per request to bypass per-IP rate limiting
    body=$(curl -s --max-time 4 -H "X-Forwarded-For: 10.0.$((i % 256)).1" "$BASE/" 2>/dev/null || true)
    if echo "$body" | grep -q '"3000"'; then
        count_3000=$((count_3000 + 1))
    elif echo "$body" | grep -q '"3001"'; then
        count_3001=$((count_3001 + 1))
    elif echo "$body" | grep -q '"3002"'; then
        count_3002=$((count_3002 + 1))
    else
        count_other=$((count_other + 1))
    fi
done

info "Backend :3000 hits: $count_3000"
info "Backend :3001 hits: $count_3001"
info "Backend :3002 hits: $count_3002"
[[ $count_other -gt 0 ]] && warn "Unidentified responses: $count_other"

# All three backends must have received requests
[[ $count_3000 -gt 0 ]] \
    && pass "Backend :3000 received traffic" \
    || fail "Backend :3000 received 0 requests — not in rotation"
[[ $count_3001 -gt 0 ]] \
    && pass "Backend :3001 received traffic" \
    || fail "Backend :3001 received 0 requests — not in rotation"
[[ $count_3002 -gt 0 ]] \
    && pass "Backend :3002 received traffic" \
    || fail "Backend :3002 received 0 requests — not in rotation"

# Round-robin: each backend should get REQUESTS/3 ± TOLERANCE
expected=$((REQUESTS / 3))
for label in "3000:$count_3000" "3001:$count_3001" "3002:$count_3002"; do
    name="${label%%:*}"
    count="${label##*:}"
    diff=$(( count > expected ? count - expected : expected - count ))
    if [[ $diff -le $TOLERANCE ]]; then
        pass "Round-robin :$name got $count requests (expected ~$expected, diff=$diff ≤ $TOLERANCE)"
    else
        fail "Round-robin :$name got $count requests (expected ~$expected, diff=$diff > tolerance=$TOLERANCE)"
    fi
done

# ── Optional: Weighted LB test (set TEST_WEIGHTED=1 and use centauri.lb-test.yml) ──
if [[ "${TEST_WEIGHTED:-0}" == "1" ]]; then
    header "Weighted Load Balancer Test (3:2:1)"
    BASE_W="http://${HTTP_HOST:-localhost}:${WEIGHTED_PORT:-8000}"
    info "Sending $REQUESTS requests to $BASE_W (weights 3:2:1)"

    wc_3000=0; wc_3001=0; wc_3002=0
    for i in $(seq 1 "$REQUESTS"); do
        body=$(curl -s --max-time 4 -H "X-Forwarded-For: 10.1.$((i % 256)).1" "$BASE_W/" 2>/dev/null || true)
        if echo "$body" | grep -q '"3000"'; then wc_3000=$((wc_3000 + 1))
        elif echo "$body" | grep -q '"3001"'; then wc_3001=$((wc_3001 + 1))
        elif echo "$body" | grep -q '"3002"'; then wc_3002=$((wc_3002 + 1))
        fi
    done

    exp_3000=$((REQUESTS * 3 / 6))
    exp_3001=$((REQUESTS * 2 / 6))
    exp_3002=$((REQUESTS * 1 / 6))
    wtol=$((TOLERANCE + 3))

    info "Backend :3000 hits: $wc_3000 (expected ~$exp_3000)"
    info "Backend :3001 hits: $wc_3001 (expected ~$exp_3001)"
    info "Backend :3002 hits: $wc_3002 (expected ~$exp_3002)"

    for entry in "3000:$wc_3000:$exp_3000" "3001:$wc_3001:$exp_3001" "3002:$wc_3002:$exp_3002"; do
        name=$(echo "$entry" | cut -d: -f1)
        cnt=$(echo "$entry"  | cut -d: -f2)
        exp=$(echo "$entry"  | cut -d: -f3)
        diff=$(( cnt > exp ? cnt - exp : exp - cnt ))
        [[ $diff -le $wtol ]] \
            && pass "Weighted :$name got $cnt (expected ~$exp, diff=$diff ≤ $wtol)" \
            || fail "Weighted :$name got $cnt (expected ~$exp, diff=$diff > tolerance=$wtol)"
    done
fi

summary
