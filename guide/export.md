# llmd export

Export documents from the store to a filesystem directory.

## Usage

```
llmd export [flags] <prefix> <dir>
```

## Flags

| Flag | Description |
|------|-------------|
| `--overwrite` | Overwrite existing files |

## Examples

```bash
# Export all documents under a prefix
llmd export notes/ ./backup/

# Overwrite existing files
llmd export --overwrite notes/ ./backup/
```

## Notes

- Document paths are preserved as directory structure within the target directory.
- Exported files are written as `.md` files.
- Existing files are not overwritten unless `--overwrite` is used.
