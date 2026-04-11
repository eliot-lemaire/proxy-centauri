#!/usr/bin/env bash
# scripts/lib/common.sh — shared utilities for Proxy Centauri test scripts
# Source this file at the top of every test script.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    printf "${GREEN}  [PASS]${RESET} %s\n" "$1"
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    printf "${RED}  [FAIL]${RESET} %s\n" "$1"
    if [[ -n "${2:-}" ]]; then
        printf "         got: %s\n" "$2"
    fi
}

info() {
    printf "${BLUE}  [INFO]${RESET} %s\n" "$1"
}

warn() {
    printf "${YELLOW}  [WARN]${RESET} %s\n" "$1"
}

header() {
    printf "\n${BOLD}=== %s ===${RESET}\n" "$1"
}

# wait_for_port HOST PORT [TIMEOUT_SECS]
# Polls TCP until the port is open or timeout expires. Returns 0 on success, 1 on timeout.
wait_for_port() {
    local host="$1" port="$2" timeout="${3:-30}"
    local elapsed=0
    while ! nc -z "$host" "$port" 2>/dev/null; do
        if [[ $elapsed -ge $timeout ]]; then
            return 1
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
    return 0
}

# wait_for_http URL [TIMEOUT_SECS]
# Polls HTTP GET until a non-zero response code arrives or timeout expires.
wait_for_http() {
    local url="$1" timeout="${2:-30}"
    local elapsed=0
    while true; do
        local code
        code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 "$url" 2>/dev/null)
        if [[ "$code" =~ ^[2-5][0-9][0-9]$ ]]; then
            return 0
        fi
        if [[ $elapsed -ge $timeout ]]; then
            return 1
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
}

# summary — print final pass/fail tally and exit with appropriate code
summary() {
    local total=$((PASS_COUNT + FAIL_COUNT))
    printf "\n${BOLD}Results: %d/%d passed${RESET}\n" "$PASS_COUNT" "$total"
    if [[ $FAIL_COUNT -gt 0 ]]; then
        printf "${RED}%d test(s) FAILED${RESET}\n" "$FAIL_COUNT"
        exit 1
    fi
    printf "${GREEN}All tests passed.${RESET}\n"
    exit 0
}
