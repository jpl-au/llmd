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

    # --- cat --offset / --limit (AI-first line slicing) ---
    # Agents use grep to find a match, then cat with offset/limit to
    # fetch just the surrounding lines without loading the whole doc.
    cat <<'MD' | $llmd --author "smoke" write long/doc >/dev/null 2>&1
line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10
MD
    out=$($llmd cat --limit 3 long/doc 2>&1)
    if echo "$out" | grep -q "line 1" && ! echo "$out" | grep -q "line 4"; then
        log_pass "cat --limit caps output from top"
    else
        log_fail "cat --limit caps output from top: got '$out'"
    fi

    out=$($llmd cat --offset 5 --limit 2 long/doc 2>&1)
    if echo "$out" | grep -q "line 5" && echo "$out" | grep -q "line 6" && \
       ! echo "$out" | grep -q "line 4" && ! echo "$out" | grep -q "line 7"; then
        log_pass "cat --offset --limit returns a window"
    else
        log_fail "cat --offset --limit returns a window: got '$out'"
    fi

    # -n with offset keeps line numbers stable against the source.
    out=$($llmd cat --offset 5 --limit 2 -n long/doc 2>&1)
    if echo "$out" | grep -q "5  line 5"; then
        log_pass "cat -n with offset uses source line numbers"
    else
        log_fail "cat -n with offset uses source line numbers: got '$out'"
    fi

    # --- version history ---
    echo "# Updated" | $llmd --author "smoke" write docs/hello >/dev/null 2>&1
    out=$($llmd history docs/hello --json 2>&1)
    if echo "$out" | grep -q '"Number"'; then
        log_pass "history returns version data"
    else
        log_fail "history returns version data: got '$out'"
    fi

    # Default limit: bulk-write 15 versions and verify history
    # caps at 10 without -n / --all.
    for i in $(seq 1 15); do
        echo "revision $i" | $llmd --author "smoke" write churn/doc >/dev/null 2>&1
    done
    out=$($llmd history churn/doc --json 2>&1)
    count=$(echo "$out" | grep -c '"Number"')
    if [ "$count" -eq 10 ]; then
        log_pass "history defaults to 10 versions"
    else
        log_fail "history defaults to 10 versions: got $count"
    fi

    out=$($llmd history --all churn/doc --json 2>&1)
    count=$(echo "$out" | grep -c '"Number"')
    if [ "$count" -eq 15 ]; then
        log_pass "history --all returns every version"
    else
        log_fail "history --all returns every version: got $count"
    fi

    # --- ls ---
    out=$($llmd ls --json 2>&1)
    if echo "$out" | grep -q "docs/hello"; then
        log_pass "ls lists documents"
    else
        log_fail "ls lists documents: got '$out'"
    fi

    # --- grep ---
    # Basic match - default mode returns matching markdown sections,
    # not full documents, so an agent searching a long doc gets back
    # bounded chunks instead of the whole file.
    out=$($llmd grep "Updated" 2>&1)
    if echo "$out" | grep -q "docs/hello"; then
        log_pass "grep finds content"
    else
        log_fail "grep finds content: got '$out'"
    fi

    # Path on its own line followed by a colon, content underneath -
    # the AI-first output contract that grep promises.
    if echo "$out" | grep -q '^docs/hello:$'; then
        log_pass "grep formats path on its own line"
    else
        log_fail "grep formats path on its own line: got '$out'"
    fi

    # Section bounding: write a multi-section doc and verify the
    # default mode returns only the matching section, never the
    # unrelated ones.
    cat <<'MD' | $llmd --author "smoke" write specs/api >/dev/null 2>&1
# API Spec

## Overview

This is the overview section.

## Authentication

OAuth2 with PKCE.

## Errors

RFC 7807 problem+json.
MD
    out=$($llmd grep "OAuth2" 2>&1)
    if echo "$out" | grep -q "## Authentication"; then
        log_pass "grep default returns matching section"
    else
        log_fail "grep default returns matching section: got '$out'"
    fi
    if echo "$out" | grep -q "RFC 7807"; then
        log_fail "grep section bounding leaked unrelated section"
    else
        log_pass "grep section bounding excludes unrelated sections"
    fi
    if echo "$out" | grep -q "## Overview"; then
        log_fail "grep section bounding leaked Overview section"
    else
        log_pass "grep section bounding excludes Overview"
    fi

    # --lines mode: line snippets instead of sections.
    out=$($llmd grep --lines "OAuth2" 2>&1)
    if echo "$out" | grep -q "OAuth2"; then
        log_pass "grep --lines returns line snippets"
    else
        log_fail "grep --lines returns line snippets: got '$out'"
    fi

    # --full mode: whole document content per match (opt-in).
    out=$($llmd grep --full "OAuth2" 2>&1)
    if echo "$out" | grep -q "RFC 7807"; then
        log_pass "grep --full returns whole document"
    else
        log_fail "grep --full returns whole document: got '$out'"
    fi

    # -l (paths only): plain path list, no content leaked.
    out=$($llmd grep -l "OAuth2" 2>&1)
    if echo "$out" | grep -q "specs/api"; then
        log_pass "grep -l returns paths"
    else
        log_fail "grep -l returns paths: got '$out'"
    fi
    if echo "$out" | grep -q "OAuth2"; then
        log_fail "grep -l leaked content into path list"
    else
        log_pass "grep -l excludes content"
    fi

    # -c (counts): one path:N line per matching document.
    out=$($llmd grep -c "OAuth2" 2>&1)
    if echo "$out" | grep -qE 'specs/api:[0-9]+'; then
        log_pass "grep -c returns path:count format"
    else
        log_fail "grep -c returns path:count format: got '$out'"
    fi

    # JSON output is always the structured GrepHit array regardless
    # of which mode is in play - that's what agents read via --json
    # or MCP.
    out=$($llmd --json grep "OAuth2" 2>&1)
    if echo "$out" | grep -q '"Path"' && echo "$out" | grep -q '"Section"'; then
        log_pass "grep --json returns structured GrepHit"
    else
        log_fail "grep --json returns structured GrepHit: got '$out'"
    fi

    # Literal punctuation: hyphenated terms must work as searches
    # without the user having to escape FTS5 syntax. The host bridge
    # auto-quotes the query as a phrase so FTS5 tokenises it
    # consistently with the indexed document.
    cat <<'MD' | $llmd --author "smoke" write notes/hyphen >/dev/null 2>&1
# Notes

foo-bar baz
MD
    out=$($llmd grep "foo-bar" 2>&1)
    if echo "$out" | grep -q "notes/hyphen"; then
        log_pass "grep handles hyphenated literal terms"
    else
        log_fail "grep handles hyphenated literal terms: got '$out'"
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

    # --- key-based access ---
    # ls should show key alongside path.
    key=$($llmd ls 2>&1 | grep "docs/greeting" | awk '{print $1}')
    if [ -n "$key" ] && [ ${#key} -eq 9 ]; then
        log_pass "ls shows document key"
    else
        log_fail "ls shows document key: got '$key'"
    fi

    # cat by key
    out=$($llmd cat "$key" 2>&1)
    if [ -n "$out" ]; then
        log_pass "cat by key"
    else
        log_fail "cat by key: got empty output"
    fi

    # cat by key:version
    out=$($llmd cat "$key:1" 2>&1)
    if [ -n "$out" ]; then
        log_pass "cat by key:version"
    else
        log_fail "cat by key:version: got '$out'"
    fi

    # history by key
    out=$($llmd history "$key" 2>&1)
    if echo "$out" | grep -q "VER"; then
        log_pass "history by key"
    else
        log_fail "history by key: got '$out'"
    fi

    # diff by key:version
    out=$($llmd diff "$key:1" "$key:2" 2>&1)
    if echo "$out" | grep -q "@@"; then
        log_pass "diff by key:version"
    else
        log_fail "diff by key:version: got '$out'"
    fi

    # tag by key
    $llmd --author "smoke" tag "$key" smoke-key-tag >/dev/null 2>&1
    out=$($llmd tag "$key" 2>&1)
    if echo "$out" | grep -q "smoke-key-tag"; then
        log_pass "tag add + list by key"
    else
        log_fail "tag add + list by key: got '$out'"
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

    # --- diff --all vs default cap ---
    # diff on a doc with a handful of lines should not truncate.
    # (Can't easily generate a 500+ line diff in a smoke test, so
    # the default-no-truncate case is the one we pin here.)
    out=$($llmd diff docs/greeting 2>&1)
    if echo "$out" | grep -q "truncated"; then
        log_fail "small diff unexpectedly truncated: got '$out'"
    else
        log_pass "small diff is not truncated"
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

    if echo "$out" | grep -q '!llmd\.db'; then
        log_pass "gitignore allows llmd.db through"
    else
        log_fail "gitignore allows llmd.db through: got '$out'"
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
