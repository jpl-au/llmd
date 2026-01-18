# llmd link

Create and manage bidirectional links between documents.

## Usage

```bash
llmd link <document|key> [documents|keys...]
llmd link --list <document|key>
llmd link --orphan
llmd unlink <id>
llmd unlink --tag <tag>
```

Documents can be specified by path or by their 8-character key.

## Examples

```bash
# Link two documents by path
llmd link docs/api docs/auth
# Output: a1b2c3d4  docs/api -> docs/auth

# Link using keys (from llmd ls output)
llmd link abc12345 def67890

# Mix paths and keys
llmd link abc12345 docs/config docs/utils

# Link one document to multiple others
llmd link docs/main docs/config docs/utils

# Create a tagged link
llmd link --tag depends-on docs/feature docs/library

# List links for a document (shows ID)
llmd link --list docs/api
# Output:
# a1b2c3d4  docs/auth
# x9y8z7w6  docs/config [depends-on]

# List links using a key
llmd link --list abc12345

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

- Documents can be specified by path or 8-character key
- Each link has a unique 8-character ID (distinct from document keys)
- Use `llmd link --list` to see IDs, then `llmd unlink <id>` to remove
- Use `llmd unlink --tag` to remove all links with a tag at once
- Links are soft-deleted (recoverable until vacuum)
- Tags are optional and can categorise relationships
- Use `--orphan` to find disconnected documents
