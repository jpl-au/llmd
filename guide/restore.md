# llmd restore

Restore a soft-deleted document.

## Usage

```bash
llmd restore <path|key>
```

Accepts either a document path or an 8-character key. You can also use `--key <key>` to explicitly specify a version key.

## Flags

| Flag | Description |
|------|-------------|
| `-k, --key` | Restore by version key (8-char identifier) |

## Examples

```bash
# See what's deleted
llmd ls -D

# Restore a document by path
llmd restore docs/readme

# Restore a document by key (positional)
llmd restore a1b2c3d4

# Restore a document by key (explicit flag)
llmd restore --key a1b2c3d4

# Verify it's back
llmd ls
```

## Notes

- Only works on soft-deleted documents
- Fails if document was permanently deleted with `llmd vacuum`
- Restores all versions of the document
