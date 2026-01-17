# llmd revert

Revert a document to a previous version.

## Usage

```bash
llmd revert <path> <version>
llmd revert <key>
```

## Flags

See `llmd guide` for global flags. The default message is "Revert to vN".

## Examples

```bash
# See version history
llmd history docs/api

# Revert to version 3
llmd revert docs/api 3

# Revert using a key (from history output)
llmd revert abc12345

# With custom message
llmd revert docs/api 3 -m "Rolling back broken changes"

# JSON output
llmd revert docs/api 3 -o json
```

## Output

```
Reverted docs/api to v3 (now v6)
```

## JSON Output

```json
{
  "path": "docs/api",
  "reverted_to": 3,
  "new_version": 6,
  "key": "abc12345",
  "author": "james",
  "message": "Revert to v3"
}
```

## How It Works

Revert is a **forward-moving** operation. It doesn't delete history - it creates a new version with the content from the old version.

```
v1 → v2 → v3 → v4 → v5
                    ↑
          llmd revert docs/api 2
                    ↓
v1 → v2 → v3 → v4 → v5 → v6 (contains v2's content)
```

This preserves the full audit trail - you can always see what was reverted and when.

## Notes

- Creates a new version, doesn't delete existing versions
- Use `llmd history <path|key>` to find version numbers or keys
- Use `llmd cat <path|key> -v N` to inspect a version before reverting
- Fails if the document is deleted (use `llmd restore` first)
- Keys are 8-character identifiers shown in history output
