# llmd mirror

Keep a directory of `.md` files in step with the store, so editors
and agents can read and change documents as files.

## Usage

```
llmd mirror                  Pull every document (the default)
llmd mirror pull [<path>]    Pull documents, optionally under a path
llmd mirror push [<path>]    Push edits back, optionally under a path
```

## Rules

- Mirrored files live under `.llmd/<dbname>/`: `.llmd/llmd/` for the
  default store, `.llmd/llmd-docs/` for `--db docs`.
- `pull` needs no author. It writes store documents as `.md` files,
  skips unchanged files, and removes stale files whose documents have
  since been deleted or renamed.
- `push` needs an author: `--author` for agents and scripts, or the
  configured author at an interactive terminal. New and modified `.md`
  files become new document versions. Unchanged files are skipped.
  When nothing changed it says `Nothing to push`.
- Push never deletes a store document. Only pull removes files.
- A subcommand other than `pull` or `push` is an error.

## Examples

```bash
# Pull all documents to the filesystem (default)
llmd mirror

# Explicit pull
llmd mirror pull

# Pull only documents under a path
llmd mirror pull notes/

# Push filesystem changes back to the store
llmd --author alice mirror push

# Push with a different database
llmd --author alice mirror push --db docs
```

## Notes

The mirror is for editor integration and agent access, such as `@`
mentions in Claude Code. Use `import` and `export` for one-off bulk
transfers to and from arbitrary directories; see `guide import` and
`guide export`.
