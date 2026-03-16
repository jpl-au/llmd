#!/usr/bin/env bash
# smoke_telemetry.sh — Telemetry build-tag smoke tests.
# Sourced by run.sh. Expects LLMD_SRC, SCRIPT_DIR, log_pass, log_fail to be set.
#
# Tests both default (no telemetry) and telemetry-enabled builds to
# verify the build tag correctly controls diagnostic logging.

_smoke_telemetry() {
    local work_dir llmd_default llmd_telem

    llmd_default="$LLMD_BIN"
    llmd_telem="$SCRIPT_DIR/llmd-telem"

    # Build telemetry-enabled binary.
    if ! (cd "$LLMD_SRC" && go build -tags telemetry -o "$llmd_telem" .); then
        log_fail "telemetry build compiles"
        return
    fi
    log_pass "telemetry build compiles"

    # --- Default build: no telemetry file ---
    work_dir=$(mktemp -d)
    cd "$work_dir"
    $llmd_default init >/dev/null 2>&1
    $llmd_default --author "smoke" cat nonexistent 2>/dev/null
    $llmd_default ls >/dev/null 2>&1

    if [ -f ".llmd/telemetry.jsonl" ]; then
        log_fail "default build produces no telemetry file"
    else
        log_pass "default build produces no telemetry file"
    fi

    cd "$SCRIPT_DIR"
    rm -rf "$work_dir"

    # --- Telemetry build: writes entries ---
    work_dir=$(mktemp -d)
    cd "$work_dir"
    $llmd_telem init >/dev/null 2>&1
    echo "test" | $llmd_telem --author "smoke" write docs/test >/dev/null 2>&1
    $llmd_telem cat docs/test >/dev/null 2>&1
    $llmd_telem nonexistent 2>/dev/null

    if [ ! -f ".llmd/telemetry.jsonl" ]; then
        log_fail "telemetry build creates telemetry.jsonl"
        cd "$SCRIPT_DIR"
        rm -rf "$work_dir"
        rm -f "$llmd_telem"
        return
    fi
    log_pass "telemetry build creates telemetry.jsonl"

    # Check entry count: write + cat + nonexistent = 3.
    # (init cannot log because .llmd/ does not exist yet when it runs.)
    local count
    count=$(wc -l < ".llmd/telemetry.jsonl" | tr -d ' ')
    if [ "$count" -eq 3 ]; then
        log_pass "telemetry file contains exactly 3 entries"
    else
        log_fail "telemetry file contains exactly 3 entries (got $count)"
    fi

    # Check that successful commands are marked success:true.
    if grep -q '"success":true' ".llmd/telemetry.jsonl"; then
        log_pass "successful commands logged with success:true"
    else
        log_fail "successful commands logged with success:true"
    fi

    # Check that failed commands are marked success:false.
    if grep -q '"success":false' ".llmd/telemetry.jsonl"; then
        log_pass "failed commands logged with success:false"
    else
        log_fail "failed commands logged with success:false"
    fi

    # Check that source is recorded.
    if grep -q '"source":"cli"' ".llmd/telemetry.jsonl"; then
        log_pass "entries include source field"
    else
        log_fail "entries include source field"
    fi

    cd "$SCRIPT_DIR"
    rm -rf "$work_dir"
    rm -f "$llmd_telem"
}

_smoke_telemetry
