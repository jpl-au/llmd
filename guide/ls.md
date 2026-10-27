# llmd ls

List documents in the store, all of them or those under a path.

## Usage

```
llmd ls [<path>]            Key and path of each document, one per line
llmd ls -l [<path>]         Long form: key, version, author, date, path
llmd ls --tree [<path>]     Directory hierarchy (terminal only)
```

## Flags

| Flag | Description |
|------|-------------|
| `-l` | Long format: KEY, VER, AUTHOR, DATE, PATH |
| `-a` | Include soft-deleted documents |
| `-r` | Reverse sort order |
| `-t` | Sort by time, newest first |
| `--tree` | Render paths as a directory hierarchy |
| `--limit <int>` | Maximum documents to return (default 500) |
| `--all` | Return every document, no limit |
| `--since <string>` | Only documents updated after a time (`5m`, `1h`, RFC 3339) |

Short flags can be combined: `-lat` is `-l -a -t`.

## Rules

- Reads need no author.
- The path argument is a prefix: documents whose path starts with it.
- At most 500 documents are listed unless `--limit N` or `--all`, so a
  large store does not dump its whole catalogue. Nothing is printed
  when no document matches.
- Deleted documents are left out unless `-a`. In long form they are
  marked `(deleted)`.
- Piped, the default form is one line per document: the key, a space,
  the path. `-l` prints tab-separated lines with a header line.
  `--tree` falls back to the default form when piped; the tree is
  drawn only at a terminal.

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

# Directory tree view
llmd ls --tree

# Tree with deleted documents
llmd ls --tree -a

# Documents updated in the last 10 minutes
llmd ls --since 10m

# Documents updated since a specific time
llmd ls --since "2026-03-16T04:00:00Z"
```

## Notes

`--json` returns one object per document with key, path, version,
author, message, created_at and deleted. For a search by content use
`llmd grep` or `llmd find`; for a path pattern use `llmd glob`.
