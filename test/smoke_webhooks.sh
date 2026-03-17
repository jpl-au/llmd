#!/usr/bin/env bash
# smoke_webhooks.sh — Webhook delivery smoke tests.
# Sourced by run.sh. Expects LLMD_BIN, SCRIPT_DIR, log_pass, log_fail to be set.

_smoke_webhooks() {
    local work_dir llmd server_pid ready listener_log
    local base="http://localhost:5563"
    local listener="http://localhost:9999"
    work_dir=$(mktemp -d)
    llmd="$LLMD_BIN"
    listener_log="$work_dir/listener.log"

    cd "$work_dir"

    # Initialise store.
    $llmd init >/dev/null 2>&1

    # Write webhook config.
    mkdir -p .llmd
    cat > .llmd/config.yaml <<'YAML'
webhook:
  smoke:
    url: http://localhost:9999
    key: smoke-secret
YAML

    # Start the webhook listener.
    (cd "$SCRIPT_DIR/listener" && go run .) > "$listener_log" 2>&1 &

    ready=false
    for _ in $(seq 1 60); do
        if curl -s -o /dev/null -X POST "$listener" 2>/dev/null; then
            ready=true
            break
        fi
        sleep 0.2
    done

    if ! $ready; then
        log_fail "Webhook listener failed to start"
        curl -s -X DELETE "$listener/shutdown" >/dev/null 2>&1
        cd /
        rm -rf "$work_dir"
        return
    fi

    # Start the server.
    $llmd --author "webhook-smoke" serve >/dev/null 2>&1 &
    server_pid=$!

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
        curl -s -X DELETE "$listener/shutdown" >/dev/null 2>&1
        cd /
        rm -rf "$work_dir"
        return
    fi

    # --- Trigger an event via HTTP write ---
    curl -sf -X POST "$base/write/docs/webhook-test" \
        -H "Author: webhook-smoke" \
        -d "# Webhook Test" >/dev/null 2>&1

    # Wait for async delivery.
    sleep 0.5

    # --- Check listener received the event ---
    if grep -q "document.written" "$listener_log"; then
        log_pass "Webhook received document.written event"
    else
        log_fail "Webhook received document.written event: $(cat "$listener_log")"
    fi

    # --- Check auth header was sent ---
    if grep -q "auth=Bearer smoke-secret" "$listener_log"; then
        log_pass "Webhook sent Authorization header"
    else
        log_fail "Webhook sent Authorization header: $(cat "$listener_log")"
    fi

    # --- Check event payload contains path ---
    if grep -q "docs/webhook-test" "$listener_log"; then
        log_pass "Webhook payload contains document path"
    else
        log_fail "Webhook payload contains document path: $(cat "$listener_log")"
    fi

    # Clean up.
    kill "$server_pid" 2>/dev/null
    wait "$server_pid" 2>/dev/null
    curl -s -X DELETE "$listener/shutdown" >/dev/null 2>&1
    cd /
    rm -rf "$work_dir"
}

_smoke_webhooks
