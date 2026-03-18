# llmd queue

Message queue for cross-consumer coordination. Domain events and direct
messages land in a single ordered queue. Consumers poll, process front
to back, and acknowledge each message.

## Usage

```
llmd queue <subcommand> [options]
```

All subcommands require `--author`. The queue binds acknowledgements
to the caller's identity.

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `send <content>` | Send a message (broadcast or directed) |
| `ls [--limit N]` | Pending messages, oldest first |
| `peek` | Next unacknowledged message with payload |
| `ack <key>` | Acknowledge oldest pending message |
| `history [--since 5m]` | All messages including acknowledged |

`ls` and `list` are aliases.

## Flags

| Flag | Description |
|------|-------------|
| `--assign` | Direct a message to a specific consumer |
| `--limit` | Maximum messages to return |
| `--since` | Show messages after a time (e.g. `5m`, `1h`, RFC 3339) |

## Examples

### Send a direct message

```bash
# Direct to an agent
llmd --author jake queue send "review task a8f3k2m1n" --assign claude-code

# Broadcast (all consumers see it)
llmd --author jake queue send "deployment complete"

# From stdin
echo "detailed instructions" | llmd --author jake queue send --assign gemini
```

### Check pending messages

```bash
# All pending for me
llmd --author claude-code queue ls

# First 5 only
llmd --author claude-code queue ls --limit 5

# Next single message with full payload
llmd --author claude-code queue peek
```

### Acknowledge a message

```bash
# Must be the oldest pending message
llmd --author claude-code queue ack a8f3k2m1n
```

Acknowledging out of order fails. The queue is strictly ordered —
process front to back, no skipping.

### View history

```bash
# Everything
llmd --author claude-code queue history

# Last hour only
llmd --author claude-code queue history --since 1h
```

## How it works

### Two types of messages

**Domain events** are published automatically. When any mutation occurs
(document written, task moved, audit created), the event bus inserts a
queue message. No manual action needed — every store change generates
a message.

**Direct messages** are sent by humans or agents via `queue send`. These
carry free-text instructions like "review this task" or "deployment
complete."

### Directed vs broadcast

Messages with `--assign` are directed — only the named consumer sees
them in their pending queue. Messages without `--assign` are broadcast
— all consumers see them.

Work assignments should always use `--assign`. Broadcasts are
informational only (awareness, not action).

### Strict ordering

The queue is ordered by creation time. Consumers must process messages
front to back. `ack` enforces this: it rejects if the key does not
match the consumer's oldest pending message. There is no way to skip
ahead or cherry-pick.

### Per-consumer acknowledgement

Each consumer has independent progress through the queue. Claude acking
a message does not affect Gemini's pending list. Broadcast messages
require each consumer to ack independently.

### Deduplication

Domain events carry a source key derived from the event type and entity
key. If multiple processes detect the same change (e.g. the event bus
and a cross-process poller), the second insert is silently ignored.
One mutation, one message.

## Consumer loop

Every consumer — human, CLI agent, MCP agent, HTTP agent — follows
the same pattern:

1. `queue ls` or `queue peek` — what happened?
2. Read the event type and payload — what do I need to do?
3. Call domain tools — `task show`, `audit show`, `cat`, etc.
4. `queue ack` — done with this message, next.

Agents poll the queue periodically or at session start. The queue
accumulates between polls.

## Transports

| Transport | Access |
|-----------|--------|
| CLI | `llmd --author X queue ls` |
| MCP | Tool call: `queue` with args `["ls"]` |
| HTTP | `GET /queue/ls?limit=10` with `Author` header |
| SSE | `message.sent` events stream via `GET /events` |

SSE consumers get real-time notification when messages arrive, then
call back to read the full message. If the SSE connection drops, fall
back to polling — the `message_acks` table is the cursor.

## Notes

- Author is required for all queue commands.
- IDs are bare 9-character base36 keys with no prefix.
- Tables are created lazily on first queue operation.
- Vacuum clears fully-acknowledged messages only.
- Use `--json` for structured output.
