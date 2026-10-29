# llmd export

Write documents from the store to the filesystem as `.md` files.

## Usage

```
llmd export [flags] <path> [dir]
```

`<path>` is the document or documents to export. If it matches a
single document, that document is exported. If it ends with `/`, all
documents under that path are exported. `[dir]` is the target
directory, the current directory by default.

## Flags

| Flag | Description |
|------|-------------|
| `--overwrite` | Overwrite existing files (by default, existing files are skipped) |

## Rules

- Read-only on the store, so no author is needed. `<path>` is
  required.
- A single document is written by its name: `api/users` becomes
  `./users.md`. A prefix keeps the structure below it: `notes/` writes
  `notes/sub/c` as `./sub/c.md`.
- Existing files are skipped unless `--overwrite` is given. The output
  lists each file written, then `skipped N existing` when any were.

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

`export` is a one-off copy to any directory. For a directory that
stays in step with the store, see `guide mirror`; for the other
direction, `guide import`.
