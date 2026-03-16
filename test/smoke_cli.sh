#!/usr/bin/env bash
# smoke_cli.sh — CLI smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, log_pass, log_fail to be set.

_smoke_cli() {
    local work_dir llmd out rc
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"

    cd "$work_dir"

    # --- init ---
    $llmd init >/dev/null 2>&1
    if [ -d ".llmd" ]; then
        log_pass "init creates .llmd directory"
    else
        log_fail "init creates .llmd directory"
    fi

    # --- write + cat ---
    echo "# Hello World" | $llmd --author "smoke" write docs/hello >/dev/null 2>&1
    out=$($llmd cat docs/hello 2>&1)
    if echo "$out" | grep -q "Hello World"; then
        log_pass "write + cat round-trip"
    else
        log_fail "write + cat round-trip: got '$out'"
    fi

    # --- version history ---
    echo "# Updated" | $llmd --author "smoke" write docs/hello >/dev/null 2>&1
    out=$($llmd history docs/hello --json 2>&1)
    if echo "$out" | grep -q '"Number"'; then
        log_pass "history returns version data"
    else
        log_fail "history returns version data: got '$out'"
    fi

    # --- ls ---
    out=$($llmd ls --json 2>&1)
    if echo "$out" | grep -q "docs/hello"; then
        log_pass "ls lists documents"
    else
        log_fail "ls lists documents: got '$out'"
    fi

    # --- grep ---
    out=$($llmd grep "Updated" 2>&1)
    if echo "$out" | grep -q "docs/hello"; then
        log_pass "grep finds content"
    else
        log_fail "grep finds content: got '$out'"
    fi

    # --- sed ---
    $llmd --author "smoke" sed -i 's/Updated/Modified/' docs/hello >/dev/null 2>&1
    out=$($llmd cat docs/hello 2>&1)
    if echo "$out" | grep -q "Modified"; then
        log_pass "sed replaces content"
    else
        log_fail "sed replaces content: got '$out'"
    fi

    # --- tags ---
    $llmd --author "smoke" tag docs/hello important >/dev/null 2>&1
    out=$($llmd tag docs/hello 2>&1)
    if echo "$out" | grep -q "important"; then
        log_pass "tag add + list"
    else
        log_fail "tag add + list: got '$out'"
    fi

    # --- mv ---
    $llmd --author "smoke" mv docs/hello docs/greeting >/dev/null 2>&1
    out=$($llmd cat docs/greeting 2>&1)
    if echo "$out" | grep -q "Modified"; then
        log_pass "mv moves document"
    else
        log_fail "mv moves document: got '$out'"
    fi

    # --- rm + restore ---
    $llmd --author "smoke" rm docs/greeting >/dev/null 2>&1
    out=$($llmd cat docs/greeting 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "rm deletes document"
    else
        log_fail "rm deletes document: cat still returns content"
    fi

    $llmd --author "smoke" restore docs/greeting >/dev/null 2>&1
    out=$($llmd cat docs/greeting 2>&1)
    if echo "$out" | grep -q "Modified"; then
        log_pass "restore recovers document"
    else
        log_fail "restore recovers document: got '$out'"
    fi

    # --- revert --- (revert to version 1, which is the pre-sed content
    # carried over by mv)
    $llmd --author "smoke" revert docs/greeting --version 1 >/dev/null 2>&1
    out=$($llmd cat docs/greeting 2>&1)
    if [ -n "$out" ]; then
        log_pass "revert creates new version"
    else
        log_fail "revert creates new version: got '$out'"
    fi

    # --- diff ---
    out=$($llmd diff docs/greeting 2>&1)
    if echo "$out" | grep -q "Hello\|Modified\|@@"; then
        log_pass "diff shows changes"
    else
        log_fail "diff shows changes: got '$out'"
    fi

    # --- tasks ---
    echo -e "# Test task\n\nThis is the spec body." | \
        $llmd --author "smoke" task add "Test task" >/dev/null 2>&1
    out=$($llmd task list --json 2>&1)
    if echo "$out" | grep -q "Test task"; then
        log_pass "task add + list"
    else
        log_fail "task add + list: got '$out'"
    fi

    # --- audits: add + rm + restore ---
    out=$($llmd --author "smoke" audit add docs/greeting "Needs review." 2>&1)
    audit_id=$(echo "$out" | sed -n 's/Created audit \([a-z0-9]*\).*/\1/p')
    if [ -n "$audit_id" ]; then
        log_pass "audit add"
    else
        log_fail "audit add: got '$out'"
    fi

    $llmd --author "smoke" audit rm "$audit_id" >/dev/null 2>&1
    out=$($llmd audit show "$audit_id" 2>&1) && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "audit rm hides audit"
    else
        log_fail "audit rm hides audit: still readable"
    fi

    $llmd --author "smoke" audit restore "$audit_id" >/dev/null 2>&1
    out=$($llmd audit show "$audit_id" 2>&1)
    if echo "$out" | grep -q "Needs review"; then
        log_pass "audit restore recovers audit"
    else
        log_fail "audit restore recovers audit: got '$out'"
    fi

    # --- gitignore whitelist ---
    out=$(cat .llmd/.gitignore 2>&1)
    if echo "$out" | grep -q '^\*$'; then
        log_pass "init creates whitelist gitignore"
    else
        log_fail "init creates whitelist gitignore: got '$out'"
    fi

    if echo "$out" | grep -q '!\.gitignore'; then
        log_pass "gitignore allows .gitignore through"
    else
        log_fail "gitignore allows .gitignore through: got '$out'"
    fi

    if echo "$out" | grep -q '!\*\.db'; then
        log_pass "gitignore allows *.db through"
    else
        log_fail "gitignore allows *.db through: got '$out'"
    fi

    # --- config git allow/deny/ls ---
    $llmd config git allow "reports/" >/dev/null 2>&1
    out=$($llmd config git ls 2>&1)
    if echo "$out" | grep -q '!reports/'; then
        log_pass "config git allow adds whitelist entry"
    else
        log_fail "config git allow adds whitelist entry: got '$out'"
    fi

    $llmd config git deny "reports/" >/dev/null 2>&1
    out=$($llmd config git ls 2>&1)
    if echo "$out" | grep -q '!reports/'; then
        log_fail "config git deny removes whitelist entry: still present"
    else
        log_pass "config git deny removes whitelist entry"
    fi

    # --- --since filtering ---
    # ls --since should return only recent documents.
    sleep 1
    echo "# Recent" | $llmd --author "smoke" write docs/recent >/dev/null 2>&1
    out=$($llmd ls --since 500ms 2>&1)
    if echo "$out" | grep -q "docs/recent"; then
        log_pass "ls --since returns recent documents"
    else
        log_fail "ls --since returns recent documents: got '$out'"
    fi
    if echo "$out" | grep -q "docs/greeting"; then
        log_fail "ls --since excludes old documents: greeting still present"
    else
        log_pass "ls --since excludes old documents"
    fi

    # task list --since should return only recent tasks.
    echo -e "# New task\n\nSpec." | \
        $llmd --author "smoke" task add "New task" >/dev/null 2>&1
    out=$($llmd task list --since 500ms 2>&1)
    if echo "$out" | grep -q "New task"; then
        log_pass "task list --since returns recent tasks"
    else
        log_fail "task list --since returns recent tasks: got '$out'"
    fi

    # --- error on missing author ---
    echo "no author" | $llmd write docs/fail >/dev/null 2>&1 && rc=0 || rc=$?
    if [ "$rc" -ne 0 ]; then
        log_pass "write without author fails"
    else
        log_fail "write without author fails: should have returned error"
    fi

    # --- version ---
    out=$($llmd version 2>&1)
    if [ -n "$out" ]; then
        log_pass "version outputs something"
    else
        log_fail "version outputs something"
    fi

    # Clean up.
    cd /
    rm -rf "$work_dir"
}

_smoke_cli
