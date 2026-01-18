# llmd restore

Restore soft-deleted documents.

## Usage

```bash
llmd restore <path|key>...
```

Accepts document paths or 8-character keys. Multiple paths can be specified to restore several documents at once.

## Flags

| Flag | Description |
|------|-------------|
| `-k, --key` | Restore by version key (8-char identifier) |

Note: `--key` flag only works with a single path.

## Examples

```bash
# See what's deleted
llmd ls -D

# Restore a document by path
llmd restore docs/readme

# Restore multiple documents
llmd restore docs/a docs/b docs/c

# Restore a document by key (positional)
llmd restore a1b2c3d4

# Restore a document by key (explicit flag)
llmd restore --key a1b2c3d4

# JSON output (single returns object, multiple returns array)
llmd restore docs/a docs/b -o json

# Verify it's back
llmd ls
```

## Notes

- Only works on soft-deleted documents
- Fails if document was permanently deleted with `llmd vacuum`
- Restores all versions of the document
- Single path returns object, multiple paths return array (JSON output)
