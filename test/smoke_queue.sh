#!/usr/bin/env bash
# smoke_queue.sh — Queue smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, log_pass, log_fail to be set.

_smoke_queue() {
    local work_dir llmd out rc msg_key msg_key2
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"

    cd "$work_dir"
    $llmd init >/dev/null 2>&1

    # --- send broadcast ---
    out=$($llmd --author "human" queue send "deployment complete" 2>&1)
    if echo "$out" | grep -q "Sent"; then
        log_pass "queue send broadcast"
    else
        log_fail "queue send broadcast: got '$out'"
    fi
    msg_key=$(echo "$out" | sed -n 's/Sent \([a-z0-9]*\).*/\1/p')

    # --- send directed ---
    out=$($llmd --author "human" queue send "review task x" --assign claude 2>&1)
    if echo "$out" | grep -q "claude"; then
        log_pass "queue send directed"
    else
        log_fail "queue send directed: got '$out'"
    fi
    msg_key2=$(echo "$out" | sed -n 's/Sent \([a-z0-9]*\).*/\1/p')

    # --- ls shows pending for claude (broadcast + directed) ---
    out=$($llmd --author "claude" queue ls 2>&1)
    if echo "$out" | grep -q "$msg_key" && echo "$out" | grep -q "$msg_key2"; then
        log_pass "queue ls shows both messages for claude"
    else
        log_fail "queue ls shows both messages for claude: got '$out'"
    fi

    # --- ls for gemini only shows broadcast ---
    out=$($llmd --author "gemini" queue ls 2>&1)
    if echo "$out" | grep -q "$msg_key"; then
        if echo "$out" | grep -q "$msg_key2"; then
            log_fail "queue ls filters directed: gemini sees claude's message"
        else
            log_pass "queue ls filters directed messages"
        fi
    else
        log_fail "queue ls filters directed: broadcast not visible to gemini"
    fi

    # --- peek returns oldest ---
    out=$($llmd --author "claude" queue peek 2>&1)
    if echo "$out" | grep -q "$msg_key"; then
        log_pass "queue peek returns oldest"
    else
        log_fail "queue peek returns oldest: got '$out'"
    fi

    # --- ack out of order fails ---
    $llmd --author "claude" queue ack "$msg_key2" >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "queue ack rejects out-of-order"
    else
        log_fail "queue ack rejects out-of-order: should have failed"
    fi

    # --- ack in order succeeds ---
    $llmd --author "claude" queue ack "$msg_key" >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "$rc" -eq 0 ]; then
        log_pass "queue ack oldest succeeds"
    else
        log_fail "queue ack oldest succeeds: got rc=$rc"
    fi

    # --- ack second message ---
    $llmd --author "claude" queue ack "$msg_key2" >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "$rc" -eq 0 ]; then
        log_pass "queue ack second succeeds"
    else
        log_fail "queue ack second succeeds: got rc=$rc"
    fi

    # --- no more pending ---
    out=$($llmd --author "claude" queue ls 2>&1)
    if echo "$out" | grep -q "No pending"; then
        log_pass "queue empty after acking all"
    else
        log_fail "queue empty after acking all: got '$out'"
    fi

    # --- history shows all messages ---
    out=$($llmd --author "claude" queue history 2>&1)
    if echo "$out" | grep -q "$msg_key" && echo "$out" | grep -q "$msg_key2"; then
        log_pass "queue history shows all messages"
    else
        log_fail "queue history shows all messages: got '$out'"
    fi

    # --- author required ---
    $llmd queue ls >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "queue ls without author fails"
    else
        log_fail "queue ls without author fails: should have returned error"
    fi

    # --- event bus integration: domain events land in queue ---
    echo "# Test doc" | $llmd --author "smoke" write docs/test >/dev/null 2>&1
    out=$($llmd --author "observer" queue ls 2>&1)
    if echo "$out" | grep -q "document.written"; then
        log_pass "domain events land in queue"
    else
        log_fail "domain events land in queue: got '$out'"
    fi

    # Clean up.
    cd /
    rm -rf "$work_dir"
}

_smoke_queue
