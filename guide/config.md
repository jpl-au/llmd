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
- `llmd config <key> <value>` refuses a value it cannot use. A value
  edited into the file by hand that cannot be used is reported as a
  warning on every command, and that key's default is used until it is
  fixed; keys the file leaves out take their defaults. Unknown keys in
  the file are ignored.
- Durations are written like `30s`, `2m` or `1h`.
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
| `queue.retention` | How long `vacuum` keeps queue messages: a number of days such as `30d` (default), a duration such as `12h`, or `0` to keep every message |
| `hook.limit` | Pending queue messages the session-start hook prints into an agent's context: `10` (default), or `0` for all of them |
| `server.read_header_timeout` | Time a client has to send its request headers (default `10s`) |
| `server.read_timeout` | Time a client has to send its whole request (default `1m`) |
| `server.idle_timeout` | How long an unused keep-alive connection stays open (default `2m`) |
| `server.shutdown_timeout` | Time requests in flight get to finish when the server stops (default `5s`) |
| `server.watch_interval` | How often `serve` looks for changes other processes made, to send as `store.changed` (default `1s`) |
| `git.timeout` | How long one git command may run before it is stopped (default `2m`) |
| `database.busy_timeout` | How long a write waits for another process's write before failing (default `5s`). Raise it when several agents write to one store at once |
| `database.cache_size` | Most page cache, in MB, each database connection may hold; used only as pages are read (default `64`) |
| `agent.worktree` | Whether spawned agents get their own git worktree: `true` (default) or `false` for the main checkout. `task start --worktree`/`--no-worktree` override it |
| `agent.harness` | The harness that runs agents on this computer, in place of the one the store names for each agent (`agent harness`). Empty, the default, leaves the choice to the store |
| `display.theme` | Colours for markdown rendered at a terminal: `dark` (default) for a dark background, `light` for a light one |
| `database.mmap_size` | How much of each database file, in MB, is memory-mapped (default `256`); `0` turns memory-mapping off |
| `database.synchronous` | How often writes are flushed to disk: `full` after every write, `normal` (default) at checkpoints, or `off` never. With `normal`, a power failure or operating system crash can lose the last writes but leaves the database intact. With `off`, it can corrupt the database, so keep `off` for stores you can rebuild, such as in CI. A crash of llmd itself loses nothing with any value |

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

queue:
  retention: 7d

hook:
  limit: 5

git:
  timeout: 5m

database:
  busy_timeout: 15s
```

## Git integration

llmd manages `.llmd/.gitignore` using a whitelist approach: everything
inside `.llmd/` is ignored by default, and specific files are allowed
through. This means generated files (telemetry, mirror output, SQLite
temp files) are automatically excluded without maintaining a blocklist.

The default `.llmd/.gitignore` created by `llmd init`:

```
# Ignore everything by default
*

# Allow the mirrored documents and this file
!.gitignore
!llmd/
!llmd/**
```

`config git allow` on a directory (`reports/`) also allows everything
under it (`reports/**`), so its files are committed, not just the
directory.

Only the mirrored documents (`.llmd/llmd/`, written by `llmd mirror`)
and the gitignore itself are committed. The databases stay local: a
binary database cannot be merged when two people change it, and
mirrored `.md` files can. To commit the database instead, run
`llmd config git allow llmd.db`.

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
