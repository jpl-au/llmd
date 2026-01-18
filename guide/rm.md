# llmd rm

Soft delete documents (recoverable).

## Usage

```bash
llmd rm <path|key>...
llmd rm -r <path>
```

Accepts document paths or 8-character keys. When given a key, deletes only that specific version. When given a path, soft-deletes the entire document. Multiple paths can be specified to delete several documents at once.

## Flags

| Flag | Description |
|------|-------------|
| `-k, --key` | Delete by version key (8-char identifier) |
| `-r, --recursive` | Delete all documents under path |
| `--version` | Delete only a specific version |

Note: `--key` and `--version` flags only work with a single path.

## Examples

```bash
# Delete a document by path
llmd rm docs/old-readme

# Delete multiple documents
llmd rm docs/a docs/b docs/c

# Delete a specific version by key (positional)
llmd rm a1b2c3d4

# Delete a specific version by key (explicit flag)
llmd rm --key a1b2c3d4

# Delete all documents under a path
llmd rm -r docs/archive/

# Delete a specific version by path and version number
llmd rm --version 3 docs/api

# JSON output (single returns object, multiple returns array)
llmd rm docs/a docs/b -o json

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
- Single path returns object, multiple paths return array (JSON output)
