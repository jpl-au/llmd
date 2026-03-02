# llmd ls

List documents in the store.

## Usage

```
llmd ls [flags] [<prefix>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-l` | Long format: shows VER, AUTHOR, DATE, PATH columns |
| `-a` | Include soft-deleted documents |
| `-t` | Sort by time, newest first |
| `-r` | Reverse sort order |
| `--tree` | Render paths as a directory hierarchy |

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

## Notes

- Without `-l` or `--tree`, output is one path per line.
- `--tree` renders a styled directory hierarchy (falls back to flat paths when piped).
- The prefix argument filters to documents whose path starts with the given string.
