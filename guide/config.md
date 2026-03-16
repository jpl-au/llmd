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
| `--global` | Write to `~/.llmd/config` instead of `.llmd/config` |

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
```

## Configuration keys

| Key | Description |
|-----|-------------|
| `author` | Default author for interactive terminal use |
| `serve_addr` | HTTP server listen address (default `localhost:5563`) |

Two configuration files are consulted: global (`~/.llmd/config`) and
local (`.llmd/config`). Local values override global values.

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
