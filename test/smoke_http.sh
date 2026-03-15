#!/usr/bin/env bash
# smoke_http.sh — HTTP API smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, log_pass, log_fail to be set.

_smoke_http() {
    local work_dir llmd out code server_pid ready
    local base="http://localhost:8080"
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"

    cd "$work_dir"

    # Initialise store and seed data.
    $llmd init >/dev/null 2>&1
    echo "# Smoke Doc" | $llmd --author "smoke" write docs/smoke >/dev/null 2>&1

    # Start the server (uses default localhost:8080).
    $llmd --author "http-smoke" serve >/dev/null 2>&1 &
    server_pid=$!

    # Wait for the server to be ready.
    ready=false
    for _ in $(seq 1 30); do
        if curl -s "$base/ls" >/dev/null 2>&1; then
            ready=true
            break
        fi
        sleep 0.1
    done

    if ! $ready; then
        log_fail "HTTP server failed to start"
        kill "$server_pid" 2>/dev/null
        wait "$server_pid" 2>/dev/null
        cd /
        rm -rf "$work_dir"
        return
    fi

    # --- GET /ls ---
    out=$(curl -sf "$base/ls" 2>&1)
    if echo "$out" | grep -q "docs/smoke"; then
        log_pass "GET /ls returns documents"
    else
        log_fail "GET /ls returns documents: got '$out'"
    fi

    # --- GET /cat/path ---
    out=$(curl -sf "$base/cat/docs/smoke" 2>&1)
    if echo "$out" | grep -q "Smoke Doc"; then
        log_pass "GET /cat returns content"
    else
        log_fail "GET /cat returns content: got '$out'"
    fi

    # --- POST /write/path ---
    out=$(curl -sf -X POST "$base/write/docs/http-test" \
        -H "Author: http-smoke" \
        -H "Message: created via HTTP" \
        -d "# HTTP Test" 2>&1)
    if echo "$out" | grep -qi "wrote\|http-test"; then
        log_pass "POST /write creates document"
    else
        log_fail "POST /write creates document: got '$out'"
    fi

    # Verify the write persisted.
    out=$(curl -sf "$base/cat/docs/http-test" 2>&1)
    if echo "$out" | grep -q "HTTP Test"; then
        log_pass "POST /write content round-trip"
    else
        log_fail "POST /write content round-trip: got '$out'"
    fi

    # --- GET /grep?q= ---
    out=$(curl -sf "$base/grep?q=Smoke" 2>&1)
    if echo "$out" | grep -q "docs/smoke"; then
        log_pass "GET /grep searches content"
    else
        log_fail "GET /grep searches content: got '$out'"
    fi

    # --- Output: json header ---
    out=$(curl -sf -H "Output: json" "$base/cat/docs/smoke" 2>&1)
    if echo "$out" | grep -q '"'; then
        log_pass "Output: json header returns JSON"
    else
        log_fail "Output: json header returns JSON: got '$out'"
    fi

    # --- 404 for missing document ---
    code=$(curl -s -o /dev/null -w "%{http_code}" "$base/cat/docs/nonexistent" 2>&1)
    if [ "$code" = "404" ]; then
        log_pass "GET /cat missing doc returns 404"
    else
        log_fail "GET /cat missing doc returns 404: got $code"
    fi

    # --- POST /rm ---
    out=$(curl -sf -X POST "$base/rm/docs/http-test" \
        -H "Author: http-smoke" 2>&1)
    if echo "$out" | grep -qi "deleted\|http-test"; then
        log_pass "POST /rm deletes document"
    else
        log_fail "POST /rm deletes document: got '$out'"
    fi

    # Clean up.
    kill "$server_pid" 2>/dev/null
    wait "$server_pid" 2>/dev/null
    cd /
    rm -rf "$work_dir"
}

_smoke_http
