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

## Examples

```bash
# Import all .md files from a directory
llmd import ./docs/

# Import under a path prefix
llmd import --prefix project/docs ./docs/

# Preview without importing
llmd import --dry-run ./docs/
```

## Notes

- Only `.md` files are imported; other file types are skipped.
- File paths relative to the import directory become document paths in the store.
- Unchanged files are skipped. Identical content never creates a new
  version, so there is no way to force one.
