# llmd export

Export documents from the store as files on your filesystem. Each
document is written as a `.md` file, preserving its path structure.

## Usage

```
llmd export [flags] <path> [dir]
```

`<path>` is the document (or documents) to export. If the path matches a
single document, that document is exported. If it ends with `/`, all
documents under that path are exported.

`[dir]` is the target directory. Defaults to the current directory.

## Flags

| Flag | Description |
|------|-------------|
| `--overwrite` | Overwrite existing files (by default, existing files are skipped) |

## Examples

```bash
# Export a single document to the current directory
llmd export my-doc

# Export all documents under notes/ to the current directory
llmd export notes/

# Export to a specific directory
llmd export notes/ ./backup

# Overwrite files that already exist
llmd export --overwrite notes/ ./backup
```

## Notes

- Document paths become file paths within the target directory
  (e.g. `api/users` is written to `./api/users.md`).
- Files are written as `.md` files.
- Existing files are skipped unless `--overwrite` is used.
