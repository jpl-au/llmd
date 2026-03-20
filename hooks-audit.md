# Agent Hook Systems - Audit & Integration Design

## Purpose

AI CLI agents (Claude Code, Gemini CLI, others) provide lifecycle hooks
that fire when specific events occur during a session. These hooks are
the natural integration point between running agents and llmd's queue
infrastructure. When an agent finishes work, encounters an error, or
completes a tool call, a hook can notify llmd - updating task status,
posting to the queue, or triggering downstream work.

This document surveys the hook capabilities across agent platforms,
identifies the common abstraction, and proposes how llmd should consume
these hooks.

**Relationship to orchestration-audit.md:** The orchestration layer
spawns agents and manages task dependencies. Hooks are the mechanism
by which running agents report back. Orchestration needs hooks, but
hooks are independently useful - a single human-spawned Claude session
can use hooks to update llmd without any orchestrator running.

---

## Hook Systems by Platform

### Claude Code

The most mature hook system. Configured in `settings.json` (global,
project, or local scope).

**Events (key subset for llmd integration):**

| Event | Fires when | Matcher | llmd use case |
|-------|-----------|---------|---------------|
| `SessionStart` | Session begins/resumes | `startup`, `resume`, `compact` | Inject pending queue messages into context |
| `SessionEnd` | Session terminates | `clear`, `logout`, etc. | Update task status, flush incomplete work |
| `PreToolUse` | Before tool execution | Tool name (regex) | Gate destructive operations |
| `PostToolUse` | After tool succeeds | Tool name (regex) | Log completions, check queue after MCP calls |
| `Stop` | Claude finishes responding | - | Quality gate before marking task done |
| `TaskCompleted` | Task marked complete | - | Signal llmd that work is finished |
| `SubagentStop` | Subagent finishes | Agent type | Track parallel workstreams |

**Handler types:** `command` (shell), `http` (POST to endpoint), `prompt`
(single-turn LLM check), `agent` (multi-turn with tools).

**Data available to hooks (JSON on stdin):**
- `session_id`, `cwd`, `hook_event_name`
- `tool_name`, `tool_input` (for tool events)
- `transcript_path` (full session log)
- `agent_id`, `agent_type` (for subagent events)

**Control flow:**
- Exit 0 → proceed (stdout injected as context for SessionStart)
- Exit 2 → block action (stderr fed back to agent as guidance)
- HTTP hooks: POST event JSON, response body parsed as control decision
- `SessionStart` hooks can write to `$CLAUDE_ENV_FILE` to set session-wide
  environment variables

**Key detail for llmd:** Claude Code supports `http` hooks natively. A hook
can POST directly to `llmd serve` or to a lightweight receiver without
needing a wrapper script. The `allowedEnvVars` field controls which env
vars are interpolated into headers (e.g. auth tokens).

### Gemini CLI

Hook support is more limited and still evolving.

**Events:** `SessionStart`, `AfterTool` are the documented hook points.
Configuration is JSON-based, similar in shape to Claude Code but with
fewer event types and no matcher/filtering system.

**Handler types:** `command` (shell). No native HTTP hook type - requires
a wrapper script that calls `curl`.

**Data available:** Less structured than Claude Code. The hook receives
context but the schema is less well-documented and subject to change.

**Key detail for llmd:** Gemini hooks require a shell wrapper to POST to
llmd. The wrapper reads stdin, extracts relevant fields, and calls
`curl` or `llmd` CLI directly.

### Other CLI Agents

Most other CLI-based AI tools (Aider, Continue, Codex CLI) do not yet
have formal hook systems. The common patterns for integration are:

- **Wrapper scripts:** Spawn the agent inside a shell script that runs
  pre/post commands. The script calls `llmd queue send` or `llmd task move`
  after the agent exits.
- **Exit code inspection:** Run `claude -p "..." ; llmd task move ...`
  chained with `&&` for success-only updates.
- **Output parsing:** Capture agent stdout/stderr, parse for structured
  results, update llmd accordingly.

These are fragile compared to native hooks but work for any subprocess.

---

## Common Abstraction

Despite differences in configuration format and event naming, all agent
hook systems share the same core model:

```
Event fires → Handler receives context (JSON) → Handler acts → Optional response controls agent
```

The minimum viable integration with llmd needs three things:

1. **Identity** - which agent is this? (`--author`)
2. **Event type** - what happened? (session end, task done, tool call, error)
3. **Payload** - relevant context (task ID, output summary, error message)

### Canonical Hook Payload

Regardless of which agent platform fires the hook, the data posted to
llmd should converge on a common shape:

```json
{
  "author": "claude-code",
  "event": "task.completed",
  "task_id": "a8f3k2m1n",
  "summary": "Implemented feature X, added tests",
  "status": "completed",
  "source": "hook",
  "transcript_path": "/path/to/transcript.jsonl"
}
```

This maps directly to `llmd queue send` with `--assign` for directing
the completion notification to the original requester.

### Adapter Pattern

Each agent platform needs a thin adapter that translates its native
hook format into the canonical payload. Two approaches:

**A. Shell adapter (universal, works everywhere):**

```bash
#!/bin/bash
# .claude/hooks/llmd-notify.sh
INPUT=$(cat)
TASK_ID=$(echo "$INPUT" | jq -r '.task_id // empty')
SESSION=$(echo "$INPUT" | jq -r '.session_id')

llmd --author claude-code queue send \
  "Task $TASK_ID completed (session: $SESSION)" \
  --assign "$ORIGINATOR"
```

**B. HTTP adapter (Claude Code native, zero dependencies):**

Claude Code's `http` hook type posts directly to an endpoint. The
receiver (llmd serve, or a small Go shim) parses the platform-specific
JSON and calls the llmd SDK.

**C. llmd CLI adapter (simplest, no receiver needed):**

The hook runs `llmd` commands directly. No HTTP receiver required.
Works for any platform that supports shell command hooks.

```json
{
  "hooks": {
    "SessionEnd": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue send 'Session ended' --assign jake"
      }]
    }]
  }
}
```

This is the lightest integration - the agent calls llmd directly via
CLI. No HTTP receiver, no adapter binary, no daemon. It works because
llmd writes to SQLite, so multiple processes (the hook, the serve
instance, other agents) all see the same data.

---

## Integration with llmd Queue

### Inbound: Agent → llmd

Hooks fire → call llmd (CLI or HTTP) → message lands in queue.

| Agent event | llmd action | Queue message |
|------------|-------------|---------------|
| Session start | `queue peek` / `queue ls` | Read pending messages |
| Task completed | `queue send` + `task move` | Notify originator |
| Task failed | `queue send` + `task move` | Alert with error context |
| Tool call (post) | `audit add` | Log significant actions |
| Session end | `queue send` | Status update |

### Outbound: llmd → Agent

At session start, agents check their queue:

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "startup",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue ls --limit 5"
      }]
    }]
  }
}
```

The output is injected into the agent's context window. The agent sees
pending messages and can act on them immediately. This is the "pull"
side - no push notifications needed.

### After compaction (Claude Code specific)

Claude Code compresses context when it gets long. A `PostCompact` or
`SessionStart` hook with `"matcher": "compact"` can re-inject critical
queue state so the agent doesn't lose awareness of pending work.

---

## Hook Receiver Design

For platforms that support HTTP hooks natively (Claude Code), or for
centralising hook processing across multiple agents, a small Go
receiver is useful.

### Minimal receiver

```go
// POST /hook
// Receives agent hook payloads, translates to llmd queue/task operations.
```

Responsibilities:
- Validate auth (shared secret via header)
- Parse platform-specific JSON into canonical form
- Call llmd SDK: `queue.Send()`, `tasks.Move()`, `audits.Add()`
- Return control response if the platform supports it (e.g. Claude
  Code HTTP hooks parse the response body)

### Where it runs

Three options, simplest first:

1. **No receiver** - hooks call `llmd` CLI directly (Option C above).
   Works for all platforms. No daemon needed.
2. **Embedded in `llmd serve`** - add a `/hook` endpoint to the existing
   HTTP server. Hooks POST there. Zero new infrastructure.
3. **Standalone binary** - separate Go program for complex routing,
   retries, or multi-store fan-out.

Option 1 is sufficient for single-machine setups. Option 2 is the
natural progression when HTTP hooks are preferred over CLI calls.

---

## Platform Configuration Examples

### Claude Code - full integration

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "startup",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue ls --limit 5"
      }]
    }],
    "SessionEnd": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue send 'Session ended'"
      }]
    }],
    "PostToolUse": [{
      "matcher": "mcp__llmd.*",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue peek"
      }]
    }]
  }
}
```

### Gemini CLI - shell adapter

```json
{
  "hooks": [{
    "event": "SessionStart",
    "type": "command",
    "command": "llmd --author gemini queue ls --limit 5"
  }, {
    "event": "AfterTool",
    "type": "command",
    "command": "llmd --author gemini queue peek"
  }]
}
```

### Generic wrapper (any agent)

```bash
#!/bin/bash
# run-agent-task.sh - wraps any CLI agent with llmd bookends

TASK_ID="$1"
AUTHOR="$2"
PROMPT="$3"

llmd --author "$AUTHOR" task move "$TASK_ID" in-progress

# Run agent (substitute claude/gemini/aider/etc.)
claude -p "$PROMPT" --max-turns 50

EXIT=$?

if [ $EXIT -eq 0 ]; then
  llmd --author "$AUTHOR" task move "$TASK_ID" completed
  llmd --author "$AUTHOR" queue send "Task $TASK_ID completed" --assign jake
else
  llmd --author "$AUTHOR" task move "$TASK_ID" failed
  llmd --author "$AUTHOR" queue send "Task $TASK_ID failed (exit $EXIT)" --assign jake
fi
```

This works for any agent that doesn't have native hooks. The wrapper
is the hook.

---

## Decisions & Open Questions

### Decided

- Hooks call `llmd` CLI directly as the default integration (simplest,
  no daemon, works everywhere)
- `--author` is mandatory on all hook-issued commands (identity binding)
- SessionStart hooks inject queue state into agent context (pull model)
- No inbound push into running sessions - agents check queue at natural
  hook points (session start, after tool use, compaction)

### Open

- **Should `llmd serve` gain a `/hook` endpoint?** Useful for Claude
  Code's native HTTP hooks. Avoids shell spawning overhead. Could
  accept the canonical payload format and route to queue/task/audit
  internally.
- **Hook templates in llmd?** `llmd hook init claude` could generate
  a `.claude/settings.json` snippet with the right configuration
  pre-filled. Same for Gemini. Reduces setup friction.
- **Transcript storage?** Hooks have access to `transcript_path`. Should
  llmd ingest transcripts as documents? Useful for auditing but
  potentially large. Could store as linked attachment rather than
  inline content.
- **Compaction resilience?** Claude Code's context compaction can lose
  queue awareness mid-session. The `PostCompact`/`SessionStart` hook
  with `"matcher": "compact"` re-injects state, but the right payload
  size needs tuning (too much = wasted context, too little = lost work).
