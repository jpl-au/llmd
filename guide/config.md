# llmd config

Read or set llmd's settings, and manage which files under `.llmd/` git
commits.

## Usage

```
llmd config                          Show all settings
llmd config <key>                    Show one setting
llmd config <key> <value>            Set a setting
llmd config git ls                   List .llmd/.gitignore rules (also: config git)
llmd config git allow <pattern>      Allow a pattern to be committed
llmd config git deny <pattern>       Stop allowing a pattern
```

## Flags

| Flag | Description |
|------|-------------|
| `--global` | Write to `~/.llmd/config.yaml` instead of `.llmd/config.yaml` |

## Rules

- Settings live in YAML: `.llmd/config.yaml` for the store, or
  `~/.llmd/config.yaml` with `--global`. When a local file exists it is
  used whole; nothing is merged from the global file.
- Keys use dot notation for nested values, such as `server.addr`.
- `config author` names the person at the terminal and is used for
  their interactive commands only. Agents and scripts must not set it;
  they pass `--author` on every change instead, so history shows who
  did what. Over MCP, the `author` field of each call does the same.
- `.llmd/.gitignore` is a whitelist: everything is ignored except the
  patterns `allow` lets through. `allow` adds the `!` prefix for you;
  `deny` removes that entry.

## Examples

```bash
# Show all configuration
llmd config

# Show the author setting
llmd config author

# Set the author
llmd config author "Alice"

# Set the author globally
llmd config --global author "Alice"

# Set the server listen address
llmd config server.addr "localhost:9090"

# Set log level
llmd config log.level debug

# List current git rules
llmd config git ls

# Allow a file or pattern to be committed
llmd config git allow "reports/"

# Stop allowing a pattern
llmd config git deny "reports/"

# Allow a second database to be committed, then stop
llmd config git allow "llmd-docs.db"
llmd config git deny "llmd-docs.db"
```

## Configuration keys

| Key | Description |
|-----|-------------|
| `author` | Default author for interactive terminal use |
| `server.addr` | HTTP server listen address (default `localhost:5563`) |
| `server.token` | Bearer token the HTTP server requires on every request. Off when empty |
| `server.webhook_workers` | Workers delivering webhook events (default `4`) |
| `server.webhook_queue` | Events that may wait for a webhook worker before new ones are dropped (default `256`) |
| `log.level` | Log level: `debug`, `info`, `warn`, `error` |
| `log.format` | Log format: `text`, `json` |
| `limits.path_length` | Maximum document path length in bytes (default `1024`) |
| `limits.content_size` | Maximum document content size in bytes (default `10485760`) |
| `task.order` | Order within a board column, for the board, `review` and `task next`: `priority` (default; priority 1 first, then board position) or `position` (board position only) |

Example config.yaml:

```yaml
author: Alice

server:
  addr: localhost:9090

log:
  level: info
  format: text

limits:
  path_length: 2048
  content_size: 20971520

task:
  order: position
```

## Git integration

llmd manages `.llmd/.gitignore` using a whitelist approach: everything
inside `.llmd/` is ignored by default, and specific files are allowed
through. This means generated files (telemetry, mirror output, SQLite
temp files) are automatically excluded without maintaining a blocklist.

The default `.llmd/.gitignore` created by `llmd init`:

```
*
!llmd.db
!.gitignore
```

Only database files and the gitignore itself are committed. Everything
else is excluded.

## Author and attribution

The `config author` setting identifies the **human user** at the terminal.
It is used automatically when you run commands interactively.

LLMs and scripts must **not** rely on `config author`. They must pass
`--author` explicitly on every mutation command so that changes are
correctly attributed:

```bash
# Human at a terminal - config author is used automatically
llmd write notes/meeting

# LLM or script - must use --author
llmd --author "Claude" write notes/summary
```

When connected via MCP, the LLM must include `"author"` in every mutation
tool call. See `guide mcp` for details.
