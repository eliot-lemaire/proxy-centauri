#!/usr/bin/env bash
# scripts/test-tcp.sh
# Tests the L4 TCP tunnel by sending data and verifying the echo reply.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

TCP_HOST="${TCP_HOST:-localhost}"
TCP_PORT="${TCP_PORT:-9000}"

header "TCP Tunnel Test"

wait_for_port "$TCP_HOST" "$TCP_PORT" 30 || {
    fail "TCP gate :$TCP_PORT not reachable after 30s"
    summary
}

# 1. Simple echo
payload="hello-centauri-$$"
response=$(printf '%s' "$payload" | nc -w 3 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
if [[ "$response" == "$payload" ]]; then
    pass "TCP echo: '$payload' → '$response'"
else
    fail "TCP echo mismatch" "sent='$payload' got='$response'"
fi

# 2. Multi-line data
multiline_payload="line1-$$
line2-$$
line3-$$"
response=$(printf '%s' "$multiline_payload" | nc -w 3 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
if [[ "$response" == "$multiline_payload" ]]; then
    pass "TCP multi-line echo works"
else
    fail "TCP multi-line echo mismatch" "got='${response//
/\\n}'"
fi

# 3. Sequential connections — verify no connection state leaks
for i in 1 2 3; do
    p="conn-$i-$$"
    r=$(printf '%s' "$p" | nc -w 2 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
    if [[ "$r" == "$p" ]]; then
        pass "TCP sequential connection $i: echo OK"
    else
        fail "TCP sequential connection $i: echo mismatch" "sent='$p' got='$r'"
    fi
done

# 4. Larger payload (1 KB)
large=$(printf '%1024s' | tr ' ' 'x')
response=$(printf '%s' "$large" | nc -w 5 "$TCP_HOST" "$TCP_PORT" 2>/dev/null || true)
if [[ "${#response}" -eq 1024 ]]; then
    pass "TCP 1 KB payload echo: no truncation"
else
    fail "TCP 1 KB payload echo: sent 1024 bytes, got ${#response} bytes"
fi

summary
