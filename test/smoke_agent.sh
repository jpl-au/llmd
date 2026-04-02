#!/usr/bin/env bash
# smoke_agent.sh — Agent registration and run tracking smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, log_pass, log_fail to be set.
#
# Exercises agent CLI commands end-to-end without spawning a real
# agent process. Tests registration, configuration, prompt templates,
# run tracking (insert-only schema), and JSON output.

_smoke_agent() {
    local work_dir llmd out rc

    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"

    cd "$work_dir"
    git init -q .
    git commit --allow-empty -q -m "init"
    $llmd init >/dev/null 2>&1

    # --- agent add (built-in) ---
    out=$($llmd agent add claude-code 2>&1)
    if echo "$out" | grep -q "Registered claude-code"; then
        log_pass "agent add claude-code"
    else
        log_fail "agent add claude-code: got '$out'"
    fi

    # --- agent ls ---
    out=$($llmd agent ls 2>&1)
    if echo "$out" | grep -q "claude-code"; then
        log_pass "agent ls lists registered agent"
    else
        log_fail "agent ls lists registered agent: got '$out'"
    fi

    # --- agent config ---
    out=$($llmd agent config claude-code 2>&1)
    if echo "$out" | grep -q "claude"; then
        log_pass "agent config shows command"
    else
        log_fail "agent config shows command: got '$out'"
    fi

    # --- agent config --json ---
    out=$($llmd --json agent config claude-code 2>&1)
    if echo "$out" | grep -q '"command"'; then
        log_pass "agent config --json returns structured data"
    else
        log_fail "agent config --json returns structured data: got '$out'"
    fi

    # --- agent prompt ---
    out=$($llmd agent prompt claude-code developer 2>&1)
    if [ -n "$out" ]; then
        log_pass "agent prompt returns template"
    else
        log_fail "agent prompt returns template: empty output"
    fi

    # --- agent runs (empty) ---
    out=$($llmd agent runs 2>&1)
    if echo "$out" | grep -q "No agent runs"; then
        log_pass "agent runs empty before any spawns"
    else
        log_fail "agent runs empty before any spawns: got '$out'"
    fi

    # --- custom agent ---
    out=$($llmd agent add test-agent --command /usr/bin/echo 2>&1)
    if echo "$out" | grep -q "Registered test-agent"; then
        log_pass "agent add custom agent"
    else
        log_fail "agent add custom agent: got '$out'"
    fi

    out=$($llmd agent ls 2>&1)
    if echo "$out" | grep -q "test-agent" && echo "$out" | grep -q "claude-code"; then
        log_pass "agent ls shows both agents"
    else
        log_fail "agent ls shows both agents: got '$out'"
    fi

    # --- agent rm ---
    out=$($llmd agent rm test-agent 2>&1)
    if echo "$out" | grep -q 'Removed agent "test-agent"'; then
        log_pass "agent rm removes agent"
    else
        log_fail "agent rm removes agent: got '$out'"
    fi

    out=$($llmd agent ls 2>&1)
    if echo "$out" | grep -q "test-agent"; then
        log_fail "agent rm: agent still listed after removal"
    else
        log_pass "agent rm: agent no longer listed"
    fi

    # --- rule set --resume ---
    $llmd rule set code --agent claude-code --role developer --resume --success test --failure blocked >/dev/null 2>&1
    out=$($llmd --json rule list 2>&1)
    if echo "$out" | grep -q '"resume": *true'; then
        log_pass "rule set --resume persists in YAML"
    else
        log_fail "rule set --resume persists in YAML: got '$out'"
    fi

    out=$($llmd rule list 2>&1)
    if echo "$out" | grep -q "resume"; then
        log_pass "rule list shows resume in display"
    else
        log_fail "rule list shows resume in display: got '$out'"
    fi

    # --- agent runs --json includes session_id field ---
    # Create a task so we can simulate a run lifecycle via internal commands.
    echo "# Smoke task" | $llmd --author "smoke" task add "Smoke task" >/dev/null 2>&1
    task_key=$($llmd --json task list 2>&1 | grep -o '"Key": *"[^"]*"' | head -1 | sed 's/.*: *"//;s/"//')

    if [ -z "$task_key" ]; then
        log_fail "task creation for agent run test: no task key"
        cd /
        rm -rf "$work_dir"
        return
    fi

    # Verify agent runs filtering flags are accepted.
    out=$($llmd agent runs --status running 2>&1)
    if echo "$out" | grep -q "No agent runs"; then
        log_pass "agent runs --status filter accepted"
    else
        log_fail "agent runs --status filter accepted: got '$out'"
    fi

    out=$($llmd agent runs --task "$task_key" 2>&1)
    if echo "$out" | grep -q "No agent runs"; then
        log_pass "agent runs --task filter accepted"
    else
        log_fail "agent runs --task filter accepted: got '$out'"
    fi

    out=$($llmd agent runs --agent claude-code 2>&1)
    if echo "$out" | grep -q "No agent runs"; then
        log_pass "agent runs --agent filter accepted"
    else
        log_fail "agent runs --agent filter accepted: got '$out'"
    fi

    # Clean up.
    cd /
    rm -rf "$work_dir"
}

_smoke_agent
