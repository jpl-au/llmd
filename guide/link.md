# llmd link

Create and manage bidirectional links between documents.

## Usage

```bash
llmd link <document> [documents...]
llmd link --list <document>
llmd link --orphan
llmd unlink <id>
llmd unlink --tag <tag>
```

## Examples

```bash
# Link two documents
llmd link docs/api docs/auth
# Output: a1b2c3d4  docs/api -> docs/auth

# Link one document to multiple others
llmd link docs/main docs/config docs/utils

# Create a tagged link
llmd link --tag depends-on docs/feature docs/library

# List links for a document (shows ID)
llmd link --list docs/api
# Output:
# a1b2c3d4  docs/auth
# x9y8z7w6  docs/config [depends-on]

# List all links with a specific tag
llmd link --list --tag depends-on

# Find orphan documents (no links)
llmd link --orphan

# Remove a link by ID
llmd unlink a1b2c3d4

# Remove all links with a tag
llmd unlink --tag depends-on
```

## Flags

### link

| Flag | Short | Description |
|------|-------|-------------|
| `--tag` | `-t` | Link tag for categorisation |
| `--list` | `-l` | List links for a document |
| `--orphan` | | List documents with no links |

### unlink

| Flag | Short | Description |
|------|-------|-------------|
| `--tag` | `-t` | Remove all links with this tag |

## Output Format

List output shows link ID and target document:
```
a1b2c3d4  docs/auth
x9y8z7w6  docs/config [depends-on]
```

JSON output:
```json
[
  {
    "id": "a1b2c3d4",
    "from_path": "docs/api",
    "to_path": "docs/auth",
    "tag": "",
    "created_at": "2025-01-10T12:00:00Z"
  }
]
```

## Notes

- Each link has a unique 8-character ID
- Use `llmd link --list` to see IDs, then `llmd unlink <id>` to remove
- Use `llmd unlink --tag` to remove all links with a tag at once
- Links are soft-deleted (recoverable until vacuum)
- Tags are optional and can categorise relationships
- Use `--orphan` to find disconnected documents
