# llmd import

Bulk import Markdown files from a filesystem directory into the store.

## Usage

```
llmd import [flags] <dir>
```

## Flags

| Flag | Description |
|------|-------------|
| `--prefix` | Target path prefix for imported documents |
| `--dry-run` | Preview what would be imported without writing |
| `--force` | Re-import files even if unchanged |

## Examples

```bash
# Import all .md files from a directory
llmd import ./docs/

# Import under a path prefix
llmd import --prefix project/docs ./docs/

# Preview without importing
llmd import --dry-run ./docs/

# Force re-import of all files
llmd import --force ./docs/
```

## Notes

- Only `.md` files are imported; other file types are skipped.
- File paths relative to the import directory become document paths in the store.
- Unchanged files are skipped unless `--force` is used.
