# llmd ls

List documents in the store.

## Usage

```
llmd ls [flags] [<path>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-l` | Long format: shows VER, AUTHOR, DATE, PATH columns |
| `-a` | Include soft-deleted documents |
| `-t` | Sort by time, newest first |
| `-r` | Reverse sort order |
| `--tree` | Render paths as a directory hierarchy |
| `--limit N` | Maximum documents to return (default 500) |
| `--all` | Return every document, no limit |
| `--since` | Only show documents updated after a time (e.g. `5m`, `1h`, RFC 3339) |

Short flags can be combined: `-lat` is equivalent to `-l -a -t`.

## Examples

```bash
# List all documents
llmd ls

# Long format
llmd ls -l

# List documents under a prefix
llmd ls notes/

# Newest first, long format
llmd ls -lt

# Oldest first, long format
llmd ls -ltr

# Include deleted documents
llmd ls -la
```

```bash
# Directory tree view
llmd ls --tree

# Tree with deleted documents
llmd ls --tree -a
```

```bash
# Documents updated in the last 10 minutes
llmd ls --since 10m

# Documents updated since a specific time
llmd ls --since "2026-03-16T04:00:00Z"
```

## Notes

- Without `-l` or `--tree`, output is one path per line.
- `--tree` renders a styled directory hierarchy (falls back to flat paths when piped).
- The path argument filters to documents whose path starts with the given string.
- Defaults to the first 500 matching documents so an agent on a large
  store doesn't get the full catalogue dumped. Use `--all` or
  `--limit` to change the cap.
