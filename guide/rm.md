# llmd rm

Soft-delete a document so it leaves listings but keeps its history.

## Usage

```
llmd rm <path>
```

## Rules

- An author is required: `--author` for agents and scripts, or the
  configured author at an interactive terminal.
- The document must exist. Deleting a path that is already deleted or
  never existed is an error.
- Nothing is lost. Content and every version stay in the store until
  `llmd vacuum` purges them; `llmd restore <path>` brings the document
  back at any time before that.
- A deleted document leaves `ls`, `grep` and `cat`. `ls -a` still lists
  it, and `ls -la` marks it `(deleted)`.

## Examples

```bash
# Delete a document
llmd --author claude-code rm old-draft

# Bring it back
llmd --author claude-code restore old-draft

# Permanently purge every soft-deleted document
llmd --author alice vacuum
```

## Notes

Soft deletion is the default because agents delete things by mistake,
and because a document another document links to should not vanish
without a trace. See `guide restore` and `guide vacuum`.
