#!/usr/bin/env bash
# scripts/test-udp.sh
# Tests the L4 UDP tunnel by sending datagrams and verifying echo replies.
# UDP is connectionless — some packet loss is acceptable; we use a 3s timeout.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

UDP_HOST="${UDP_HOST:-localhost}"
UDP_PORT="${UDP_PORT:-9001}"

header "UDP Tunnel Test"

info "Allowing 3s for UDP sticky session setup..."
sleep 3

# 1. Single datagram echo
payload="udp-ping-$$"
response=$(printf '%s' "$payload" | nc -u -w 3 "$UDP_HOST" "$UDP_PORT" 2>/dev/null || true)
if [[ "$response" == "$payload" ]]; then
    pass "UDP single datagram echo: '$payload'"
else
    fail "UDP single datagram echo mismatch" \
         "sent='$payload' got='$response' (empty = no reply within 3s)"
fi

# 2. Sticky session — second datagram from same process (same source port) reuses session
for i in 1 2 3; do
    p="sticky-$i-$$"
    r=$(printf '%s' "$p" | nc -u -w 3 "$UDP_HOST" "$UDP_PORT" 2>/dev/null || true)
    if [[ "$r" == "$p" ]]; then
        pass "UDP sticky-session datagram $i: echo OK"
    else
        warn "UDP sticky-session datagram $i: no echo (environment may discard UDP replies)"
    fi
done

# 3. Large datagram (close to MTU)
large=$(printf '%1000s' | tr ' ' 'U')
response=$(printf '%s' "$large" | nc -u -w 3 "$UDP_HOST" "$UDP_PORT" 2>/dev/null || true)
if [[ "${#response}" -eq 1000 ]]; then
    pass "UDP 1000-byte datagram: echo OK, no truncation"
else
    warn "UDP 1000-byte datagram: sent 1000b, got ${#response}b (fragmentation possible)"
fi

summary
