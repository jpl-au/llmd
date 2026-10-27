# llmd events

Real-time event system for reacting to store mutations. Every write,
delete, move, tag, link, audit, and task operation emits an event.

## Event types

All 19 event types across 6 domains:

| Domain   | Event               | Fires when                      |
|----------|---------------------|---------------------------------|
| Document | `document.written`  | Document created or updated     |
| Document | `document.deleted`  | Document soft-deleted           |
| Document | `document.restored` | Deleted document restored       |
| Document | `document.moved`    | Document moved to a new path    |
| Tag      | `tag.added`         | Tag added to a document         |
| Tag      | `tag.removed`       | Tag removed from a document     |
| Link     | `link.created`      | Link between documents created  |
| Link     | `link.removed`      | Link between documents removed  |
| Audit    | `audit.created`     | New audit thread created        |
| Audit    | `audit.replied`     | Reply added to an audit thread  |
| Audit    | `audit.resolved`    | Audit marked as approved        |
| Audit    | `audit.deleted`     | Audit soft-deleted              |
| Audit    | `audit.restored`    | Deleted audit restored          |
| Task     | `task.created`      | New task created                |
| Task     | `task.moved`        | Task moved between columns      |
| Task     | `task.updated`      | Task metadata changed           |
| Task     | `task.deleted`      | Task soft-deleted               |
| Task     | `task.restored`     | Deleted task restored           |
| Store    | `store.changed`     | Another process changed a database (see below) |

## Event payload

Every event is a JSON object:

```json
{
  "type": "audit.created",
  "path": "docs/spec",
  "key": "abc123def",
  "version": 0,
  "author": "Gemini",
  "timestamp": 1773700800000,
  "metadata": {
    "assignee": "Claude",
    "status": "pending"
  }
}
```

- `type` - the event type from the table above
- `path` - document path or audit target
- `key` - entity key (document key, audit ID, task key)
- `version` - document version (documents only)
- `author` - who caused the mutation
- `timestamp` - Unix milliseconds
- `metadata` - domain-specific data (tag name, link target, audit
  status, task column, etc.)

### Changes made by other processes

The typed events above are raised by the process that makes the
change. `llmd serve` is one process; the command line, an MCP session
and a spawned agent are others, and their writes raise no event inside
the server. So the server also watches the database files, once a
second, and streams a `store.changed` event when another process has
committed to one:

```json
{"type":"store.changed","timestamp":1773700800000,"metadata":{"db":"work.db"}}
```

It says only which file changed, `llmd.db` (documents) or `work.db`
(tasks, audits, messages); reread what matters. Changes the server
made itself are not repeated this way. `store.changed` reaches SSE and
webhook consumers only; it is never queued or handed to extensions.

## Delivery mechanisms

### SSE (Server-Sent Events)

Stream events in real-time from `llmd serve`:

```bash
# All events
curl -N http://localhost:5563/events

# Filter by type
curl -N "http://localhost:5563/events?type=audit.created,task.moved"
```

Events are delivered in standard SSE format:

```
event: document.written
data: {"type":"document.written","path":"docs/hello","key":"abc123","version":1,"author":"test","timestamp":1773700737508}
```

Multiple types can be filtered with a comma-separated `type` query
parameter. Omit the parameter to receive all events.

Slow clients that fall behind have events dropped rather than blocking
the server.

### Webhook

Configure HTTP endpoints to receive events as POST requests. Every
event is broadcast to all configured webhooks.

In `.llmd/config.yaml`:

```yaml
webhook:
  my-service:
    url: http://localhost:8080/llmd-events
    key: secret-api-key
  monitoring:
    url: http://example.com/hooks
    key: another-key
```

Each event is POSTed as JSON. The `key` is sent as an `Authorization:
Bearer <key>` header.

Delivery never blocks the event bus. Events are queued and sent by a
small set of workers. A request that fails with a network error or a
5xx response is retried, waiting 1s, then 2s, then 4s and so on
between attempts. A 4xx response is not retried because the endpoint
rejected the request itself. An event is dropped, with a warning in
the log, when the retries are used up or when the queue is full
because an endpoint is too slow.

The number of workers and the queue depth are set once for the server
with `server.webhook_workers` and `server.webhook_queue`. Two optional
settings tune each endpoint:

```yaml
webhook:
  my-service:
    url: http://localhost:8080/llmd-events
    key: secret-api-key
    retries: 5      # retries after the first attempt (default 2)
    timeout: 30s    # per request (default 10s)
```

### CLI polling

CLI agents and humans poll for state changes using existing commands:

```bash
# Check the agent inbox
llmd --author claude-code audit status

# List recent audits
llmd audit list --since 5m

# List recent audits needing attention
llmd audit list --pending --assign claude-code
```

This is the natural model for CLI consumers. See `guide hooks` for
how to wire this into agent session start.

## Notes

- SSE and webhooks carry typed events for mutations made through
  `llmd serve`, and a `store.changed` event for mutations made by any
  other process. Polling with `--since` remains the way to learn what
  changed outside the server.
- The event bus is in-process and synchronous - handlers are called
  in subscription order before returning to the caller.
- Extensions can subscribe to events via the extension event handler
  interface for compile-time plugins.
