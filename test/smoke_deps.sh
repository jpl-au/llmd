#!/usr/bin/env bash
# smoke_deps.sh — Task dependency smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, log_pass, log_fail to be set.

_smoke_deps() {
    local work_dir llmd out rc t1_key t2_key t3_key
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"

    cd "$work_dir"
    $llmd init >/dev/null 2>&1

    # --- add with --depends-on ---
    out=$($llmd --author "smoke" task add "Base task" 2>&1)
    t1_key=$(echo "$out" | sed -n 's/Created task \([a-z0-9]*\).*/\1/p')
    if [ -z "$t1_key" ]; then
        log_fail "deps: create base task"
        cd / && rm -rf "$work_dir"
        return
    fi

    out=$($llmd --author "smoke" task add "Dependent task" --depends-on "$t1_key" 2>&1)
    t2_key=$(echo "$out" | sed -n 's/Created task \([a-z0-9]*\).*/\1/p')
    if [ -n "$t2_key" ]; then
        log_pass "deps: add with --depends-on"
    else
        log_fail "deps: add with --depends-on: got '$out'"
    fi

    # --- show displays dependency ---
    out=$($llmd task show "$t2_key" 2>&1)
    if echo "$out" | grep -q "$t1_key"; then
        log_pass "deps: show displays depends on"
    else
        log_fail "deps: show displays depends on: got '$out'"
    fi

    # --- chain ---
    out=$($llmd task chain "$t2_key" 2>&1)
    if echo "$out" | grep -q "$t1_key" && echo "$out" | grep -q "$t2_key"; then
        log_pass "deps: chain shows both tasks"
    else
        log_fail "deps: chain shows both tasks: got '$out'"
    fi

    # --- ready (blocked) ---
    out=$($llmd task ready "$t2_key" 2>&1)
    if echo "$out" | grep -q "blocked"; then
        log_pass "deps: ready reports blocked"
    else
        log_fail "deps: ready reports blocked: got '$out'"
    fi

    # --- ready (satisfied) ---
    echo -e "# Base task\n\nSpec content." | \
        $llmd --author "smoke" write "tasks/base-task" >/dev/null 2>&1
    $llmd --author "smoke" task move "$t1_key" done >/dev/null 2>&1
    out=$($llmd task ready "$t2_key" 2>&1)
    if echo "$out" | grep -q "ready"; then
        log_pass "deps: ready reports satisfied"
    else
        log_fail "deps: ready reports satisfied: got '$out'"
    fi

    # --- set --depends-on ---
    out=$($llmd --author "smoke" task add "Third task" 2>&1)
    t3_key=$(echo "$out" | sed -n 's/Created task \([a-z0-9]*\).*/\1/p')
    $llmd --author "smoke" task set "$t3_key" --depends-on "$t2_key" >/dev/null 2>&1
    out=$($llmd task show "$t3_key" 2>&1)
    if echo "$out" | grep -q "$t2_key"; then
        log_pass "deps: set --depends-on"
    else
        log_fail "deps: set --depends-on: got '$out'"
    fi

    # --- chain through three tasks ---
    out=$($llmd task chain "$t3_key" 2>&1)
    if echo "$out" | grep -q "$t1_key" && echo "$out" | grep -q "$t2_key" && echo "$out" | grep -q "$t3_key"; then
        log_pass "deps: chain shows three-task chain"
    else
        log_fail "deps: chain shows three-task chain: got '$out'"
    fi

    # --- cycle detection ---
    out=$($llmd --author "smoke" task set "$t1_key" --depends-on "$t3_key" 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "deps: cycle detection rejects circular dependency"
    else
        log_fail "deps: cycle detection rejects circular dependency: got '$out'"
    fi

    # --- clear dependency ---
    $llmd --author "smoke" task set "$t2_key" --depends-on "" >/dev/null 2>&1
    out=$($llmd task show "$t2_key" 2>&1)
    if echo "$out" | grep -q "Depends On"; then
        log_fail "deps: clear dependency: still shows Depends On"
    else
        log_pass "deps: clear dependency"
    fi

    # Clean up.
    cd /
    rm -rf "$work_dir"
}

_smoke_deps
