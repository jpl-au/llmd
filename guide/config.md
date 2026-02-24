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
| `--global` | Write to `~/.config/llmd/config` instead of `.llmd/config` |

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
- Two configuration files are consulted: global (`~/.config/llmd/config`) and local (`.llmd/config`).
- Local values override global values.
