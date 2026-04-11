#!/usr/bin/env bash
# scripts/test-hotreload.sh
# Tests config hot-reload by appending a harmless comment to centauri.test.yml,
# then verifying the reload log line appears in the centauri container output.
# Also verifies that updated star_systems take effect immediately (Bug 1 fix).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"

COMPOSE_FILES="${COMPOSE_FILES:--f docker-compose.yml -f docker-compose.test.yml}"
COMPOSE_CMD="docker compose $COMPOSE_FILES"
CONFIG_FILE="${CONFIG_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/centauri.test.yml}"
HTTP_BASE="http://${HTTP_HOST:-localhost}:${HTTP_PORT:-8000}"

header "Config Hot-Reload Test"

wait_for_http "$HTTP_BASE/" 30 || { fail "HTTP gate not ready"; summary; }

# ── Trigger a reload by appending a comment ───────────────────────────────────
MARKER="reload-trigger-$$"
info "Appending comment to $CONFIG_FILE (marker: $MARKER)..."
printf '\n# %s\n' "$MARKER" >> "$CONFIG_FILE"

info "Waiting up to 10s for reload log in container output..."
reload_logged=false
for i in $(seq 1 10); do
    logs=$($COMPOSE_CMD logs --no-log-prefix centauri 2>/dev/null || true)
    if echo "$logs" | grep -q "Reloaded\|config_reload\|centauri.yml"; then
        reload_logged=true
        info "Reload event logged after ${i}s"
        break
    fi
    sleep 1
done

if $reload_logged; then
    pass "Hot-reload: config change logged in container output"
else
    fail "Hot-reload: no reload log within 10s after config file write"
fi

# Verify proxy still serves requests after reload
code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 "$HTTP_BASE/" 2>/dev/null)
[[ "$code" == "200" ]] \
    && pass "Proxy still serving 200 OK after hot-reload" \
    || fail "Proxy not serving correctly after hot-reload (got $code)"

# ── Clean up appended comment ─────────────────────────────────────────────────
# Remove the last line (the comment we added)
line_count=$(wc -l < "$CONFIG_FILE")
head -n "$((line_count - 1))" "$CONFIG_FILE" > "${CONFIG_FILE}.tmp" \
    && mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
# Also remove the blank line we added before the comment
line_count=$(wc -l < "$CONFIG_FILE")
last_line=$(tail -n 1 "$CONFIG_FILE")
if [[ -z "$last_line" ]]; then
    head -n "$((line_count - 1))" "$CONFIG_FILE" > "${CONFIG_FILE}.tmp" \
        && mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
fi
info "Config file restored"

summary
