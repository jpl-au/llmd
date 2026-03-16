# llmd mirror

Sync documents between the store and filesystem. Mirrored files live
under `.llmd/<dbname>/` (e.g. `.llmd/llmd/` for the default store,
`.llmd/llmd-docs/` for `--db docs`).

## Usage

```
llmd mirror [pull|push] [<path>]
```

## Subcommands

**pull** (default) — Write store documents to the filesystem as `.md` files.
Unchanged files are skipped. Stale files (documents since deleted or renamed)
are removed from the mirror directory.

**push** — Import filesystem changes back into the store. New and modified
`.md` files are written as new document versions. Unchanged files are skipped.
Requires an author (`llmd config author` for interactive use,
or `--author` for LLMs and scripts).

## Examples

```bash
# Pull all documents to filesystem (default)
llmd mirror

# Explicit pull
llmd mirror pull

# Pull only documents under a path
llmd mirror pull notes/

# Push filesystem changes back to store
llmd mirror push

# Push with a different database
llmd mirror push --db docs
```

## Notes

- Mirror directory is derived from the database name: `.llmd/llmd/` for
  the default store, `.llmd/llmd-docs/` for `--db docs`.
- Pull removes stale files; push does not delete store documents.
- Use `import` and `export` for one-off bulk transfers to arbitrary directories.
