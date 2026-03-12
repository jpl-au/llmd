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

## Notes

- The only supported key is `author`.
- Two configuration files are consulted: global (`~/.llmd/config`) and local (`.llmd/config`).
- Local values override global values.

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
