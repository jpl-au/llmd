# Agent hooks

How to wire AI agents to react to llmd events. Each agent platform has
its own hook system - this guide shows how to connect them to llmd.

## Claude Code

### Session start - check inbox

Add a hook in `.claude/settings.json` that runs at session start to
show the agent what needs its attention:

```json
{
  "hooks": {
    "SessionStart": [{
      "type": "command",
      "command": "llmd --author claude-code audit status"
    }]
  }
}
```

The output is injected into the agent's context as the session begins.
The agent sees any pending audits assigned to it and can act on them
immediately.

### Post-tool - check after MCP calls

Use a `PostToolUse` hook to check for updates after the agent calls
llmd tools:

```json
{
  "hooks": {
    "PostToolUse": [{
      "type": "command",
      "command": "llmd --author claude-code audit status",
      "matcher": "mcp__llmd*"
    }]
  }
}
```

The `matcher` ensures the hook only fires after llmd MCP tool calls,
not after every tool use.

## Gemini CLI

### Session start - check inbox

Add a hook in your Gemini CLI hooks configuration:

```json
{
  "hooks": [{
    "event": "SessionStart",
    "type": "command",
    "command": "llmd --author gemini audit status"
  }]
}
```

### After tool use

```json
{
  "hooks": [{
    "event": "AfterTool",
    "type": "command",
    "command": "llmd --author gemini audit status"
  }]
}
```

## Generic (any agent with HTTP)

### SSE stream consumer

Any agent or wrapper script can connect to the SSE endpoint and react
to events:

```bash
curl -sf -N "http://localhost:5563/events?type=audit.created" | \
  while IFS= read -r line; do
    case "$line" in
      data:*) echo "${line#data:}" | jq . ;;
    esac
  done
```

### Webhook receiver

Point a webhook at your service and react to POSTs. Configure in
`.llmd/config.yaml`:

```yaml
webhook:
  my-agent:
    url: http://localhost:8080/llmd-events
    key: secret
```

The payload is the event JSON. Filter by `type` field in your handler.

## Polling pattern for CLI agents

For agents without persistent connections (most CLI-based agents), the
recommended pattern is:

1. **Session start** - `llmd --author <identity> audit status` to
   discover pending work
2. **Work through items** - address each pending audit or task
3. **Signal completion** - reply with `--status approved` or update
   the task status
4. **Session ends** - next session repeats from step 1

This requires no special infrastructure. The agent simply calls llmd
commands to check state and act on it.

## Notes

- `--author` is required for all agent-issued commands. Config author
  is for humans only. See `guide audit` for details.
- Hook output is injected into the agent's context window, so keep
  responses concise. `audit status` is designed for this - it returns
  a summary, not full content.
- For agents managing multiple stores, prefix the llmd command with
  a `cd` to the correct working directory, since llmd uses `.llmd/`
  relative to the current directory.
