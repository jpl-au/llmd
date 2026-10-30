# llmd mirror

Keep a directory of `.md` files in step with the store, so editors
and agents can read and change documents as files.

## Usage

```
llmd mirror                  Pull every document (the default)
llmd mirror pull [<path>]    Pull documents, optionally under a path
llmd mirror push [<path>]    Push edits back, optionally only those under a path
```

## Rules

- Mirrored files live under `.llmd/<dbname>/`: `.llmd/llmd/` for the
  default store, `.llmd/llmd-docs/` for `--db docs`.
- Each store has its own tasks, comments and agent runs: `--db docs` keeps them in `.llmd/work-docs.db`, apart from the default store's `.llmd/work.db`.
- `pull` needs no author. It writes store documents as `.md` files,
  skips unchanged files, and removes stale files whose documents have
  since been deleted or renamed.
- `push` needs an author: `--author` for agents and scripts, or the
  configured author at an interactive terminal. New and modified `.md`
  files become new document versions. Unchanged files are skipped.
  The summary counts created, updated and unchanged files; with no
  files in the mirror at all it says `Nothing to push`. A path limits
  push to that part of the mirror, as it limits pull, so `pull docs`
  then `push docs` keeps every path as it was.
- Push deletes (softly) a document whose file was removed from the
  mirror since the last pull or push, so a deletion made in git
  reaches the store. It deletes only documents it knows were in step
  with their file: one written to the store since the last pull has no
  file yet and is kept, and so is one changed in the store after its
  file was removed. `llmd restore` brings a deleted document back.
- Tags and links travel in `.meta.yaml` at the top of the mirror,
  keyed by document path, with each document's tags and outgoing
  links. `pull` writes it for every document. `push` applies what
  changed in it since the last pull or push: tags and links added to
  the file are added, those removed from it are removed, and ones made
  in the store since are kept.

  ```yaml
  docs/api:
    tags:
      - backend
    links:
      - to: docs/auth
        label: depends-on
  ```
- What the last pull or push left in step is recorded in
  `.llmd/<dbname>.manifest`, beside the mirror directory. It is local
  to the checkout and never committed. With no manifest, as in a fresh
  clone, push deletes nothing.
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

# Share documents through git: the default .llmd/.gitignore commits
# the mirror and keeps the database local
llmd mirror pull && git add .llmd && git commit -m "Update docs"
git pull && llmd --author alice mirror push
```

## Notes

The mirror is for editor integration and agent access, such as `@`
mentions in Claude Code, and it is how documents are shared through
git: the databases stay local, and the mirrored `.md` files are what
is committed and merged. Use `import` and `export` for one-off bulk
transfers to and from arbitrary directories; see `guide import` and
`guide export`.
