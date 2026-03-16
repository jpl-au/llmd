# llmd config

View or set configuration values.

## Usage

```
llmd config
llmd config <key>
llmd config <key> <value>
```

## Flags

| Flag | Description |
|------|-------------|
| `--global` | Write to `~/.llmd/config.yaml` instead of `.llmd/config.yaml` |

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
```

## Configuration keys

Keys use dot notation for nested values. The config file is YAML.

| Key | Description |
|-----|-------------|
| `author` | Default author for interactive terminal use |
| `server.addr` | HTTP server listen address (default `localhost:5563`) |
| `log.level` | Log level: `debug`, `info`, `warn`, `error` |
| `log.format` | Log format: `text`, `json` |
| `limits.path_length` | Maximum document path length in bytes (default `1024`) |
| `limits.content_size` | Maximum document content size in bytes (default `10485760`) |

## Config file

Configuration is stored as YAML in `.llmd/config.yaml` (local) or
`~/.llmd/config.yaml` (global). If a local file exists, it is used
entirely — there is no merge with the global file.

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
```

## Git integration

llmd manages `.llmd/.gitignore` using a whitelist approach: everything
inside `.llmd/` is ignored by default, and specific files are allowed
through. This means generated files (telemetry, mirror output, SQLite
temp files) are automatically excluded without maintaining a blocklist.

The default `.llmd/.gitignore` created by `llmd init`:

```
*
!*.db
!.gitignore
```

Only database files and the gitignore itself are committed. Everything
else is excluded.

### Managing git rules

```bash
# List current rules
llmd config git ls

# Allow a file or pattern to be committed
llmd config git allow "reports/"

# Stop allowing a pattern
llmd config git deny "reports/"
```

The `allow` command adds a `!` prefix automatically — you pass the bare
pattern. `deny` removes the corresponding `!pattern` entry.

### Examples

```bash
# Allow a second database to be committed
llmd config git allow "llmd-docs.db"

# See what rules are in place
llmd config git ls

# Stop committing a database
llmd config git deny "llmd-docs.db"
```

## Author and attribution

The `config author` setting identifies the **human user** at the terminal.
It is used automatically when you run commands interactively.

LLMs and scripts must **not** rely on `config author`. They must pass
`--author` explicitly on every mutation command so that changes are
correctly attributed:

```bash
# Human at a terminal — config author is used automatically
llmd write notes/meeting

# LLM or script — must use --author
llmd --author "Claude" write notes/summary
```

When connected via MCP, the LLM must include `"author"` in every mutation
tool call. See `llmd guide mcp` for details.
