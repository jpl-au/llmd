# llmd rm

Soft delete a document (recoverable).

## Usage

```bash
llmd rm <path|key>
llmd rm -r <path>
```

Accepts a document path or an 8-character key. When given a key, deletes only that specific version. When given a path, soft-deletes the entire document. You can also use `--key <key>` to explicitly specify a version key.

## Flags

| Flag | Description |
|------|-------------|
| `-k, --key` | Delete by version key (8-char identifier) |
| `-r, --recursive` | Delete all documents under path |
| `--version` | Delete only a specific version |

## Examples

```bash
# Delete a document by path
llmd rm docs/old-readme

# Delete a specific version by key (positional)
llmd rm a1b2c3d4

# Delete a specific version by key (explicit flag)
llmd rm --key a1b2c3d4

# Delete all documents under a path
llmd rm -r docs/archive/

# Delete a specific version by path and version number
llmd rm --version 3 docs/api

# View deleted documents
llmd ls -D

# Restore if needed
llmd restore docs/old-readme
```

## Notes

- Soft delete only - document can be restored with `llmd restore`
- All versions are preserved
- Use `llmd vacuum` to permanently delete
- No confirmation required (soft delete is the safety net)
- Use `-r` to delete all documents under a path prefix
