# llmd ls

List documents in the store.

## Usage

```bash
llmd ls [prefix]
```

The optional `prefix` filters documents by path prefix (e.g., `docs/` lists direct children under `docs/`). By default, only direct children are shown. Use `-R` to list all nested documents recursively.

## Flags

| Flag | Description |
|------|-------------|
| `-R, --recursive` | List subdirectories recursively |
| `-l, --long` | Long format (version, key, size, date, author) |
| `-t, --tree` | Display as tree |
| `-s, --sort` | Sort by: `name`, `time` |
| `-r, --reverse` | Reverse sort order |
| `-D, --deleted` | Show deleted documents only |
| `-A, --all` | Show all (including deleted) |
| `--tag` | Filter by tag |

See `llmd guide` for global flags.

## Examples

```bash
# List all documents
llmd ls

# Long format with metadata
llmd ls -l

# List under prefix
llmd ls docs/

# Tree view
llmd ls -t

# Deleted only
llmd ls -D

# All including deleted
llmd ls -A

# Sort by name
llmd ls -s name

# Sort by time (newest first)
llmd ls -s time

# Sort by time (oldest first)
llmd ls -s time -r

# List all documents recursively
llmd ls -R

# JSON output
llmd ls -o json
```

## Output Formats

Default:
```
KEY       PATH
a1b2c3d4  docs/readme
e5f6g7h8  docs/api/auth
i9j0k1l2  notes/todo
```

Long (`-l`):
```
 VER  KEY       SIZE  UPDATED           AUTHOR  PATH
   3  a1b2c3d4  1.2K  2024-01-15 10:30  james   docs/readme
   1  e5f6g7h8   542B  2024-01-14 09:00  claude  docs/api/auth
```

Tree (`-t`):
```
├── docs/
│   ├── readme
│   └── api/
│       └── auth
└── notes/
    └── todo
```

JSON (`-o json`):
```json
[
  {
    "key": "a1b2c3d4",
    "path": "docs/readme",
    "version": 3,
    "author": "james",
    "message": "Initial commit",
    "created_at": "2024-01-15T10:30:00Z",
    "size": 1234
  },
  {
    "key": "e5f6g7h8",
    "path": "docs/api/auth",
    "version": 1,
    "author": "claude",
    "message": "",
    "created_at": "2024-01-14T09:00:00Z",
    "size": 542
  }
]
```
