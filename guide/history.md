# llmd history

Show the version history of a document.

Defaults to the 10 most recent versions so an agent running history
on a heavily-edited document doesn't dump hundreds of rows into its
context window. Use `-n` to pick a different cap, or `--all` to show
every version.

## Usage

```
llmd history [flags] <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `-n N` | Limit to the most recent N versions (default 10) |
| `--all` | Show every version, no limit |

## Examples

```bash
# Recent 10 versions (default)
llmd history notes/meeting

# Last 3 versions
llmd history -n3 notes/meeting

# Every version
llmd history --all notes/meeting

# JSON output
llmd history --json notes/meeting
```

## Notes

- Output is a table with columns: Version, Author, Date, Message.
- Versions are numbered from 1 (oldest) upwards; newest are shown first.
