#!/usr/bin/env bash
# scripts/test-http.sh
# Tests HTTP proxy behavior: status codes, header forwarding, POST proxying.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"

header "HTTP Proxy Tests"

wait_for_http "$BASE/" 30 || { fail "HTTP gate not ready after 30s"; summary; }

# 1. Basic GET returns 200
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/")
[[ "$code" == "200" ]] && pass "GET / → 200 OK" || fail "GET / → expected 200, got $code"

# 2. Various paths are proxied (any 2xx–4xx means it reached a backend)
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/some/path")
[[ "$code" =~ ^[2-4][0-9][0-9]$ ]] \
    && pass "GET /some/path → $code (proxied to backend)" \
    || fail "GET /some/path → unexpected code $code"

# 3. X-Forwarded-For header is set and echoed back
# ealen/echo-server reflects all request headers in its JSON response body
body=$(curl -s "$BASE/" 2>/dev/null)
if echo "$body" | grep -qi "x-forwarded-for"; then
    pass "X-Forwarded-For header present in proxied response"
else
    fail "X-Forwarded-For header not reflected in response body"
fi

# 4. Custom header passes through to backend
body=$(curl -s -H "X-Test-Token: centauri-check-$$" "$BASE/" 2>/dev/null)
if echo "$body" | grep -qi "centauri-check"; then
    pass "Custom request header passes through to backend"
else
    fail "Custom request header not reflected in backend response"
fi

# 5. POST request is proxied
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -d '{"test":true}' "$BASE/")
[[ "$code" =~ ^[2-4][0-9][0-9]$ ]] \
    && pass "POST / → $code (proxied)" \
    || fail "POST / → unexpected $code"

# 6. PUT request is proxied
code=$(curl -s -o /dev/null -w "%{http_code}" -X PUT -d "data=1" "$BASE/resource")
[[ "$code" =~ ^[2-4][0-9][0-9]$ ]] \
    && pass "PUT /resource → $code (proxied)" \
    || fail "PUT /resource → unexpected $code"

# 7. Query parameters are forwarded
body=$(curl -s "$BASE/?foo=bar&baz=qux" 2>/dev/null)
if echo "$body" | grep -qi "foo"; then
    pass "Query parameters forwarded to backend"
else
    warn "Query parameters may not be reflected by this backend"
fi

summary
