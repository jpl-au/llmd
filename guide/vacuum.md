# llmd vacuum

Permanently remove soft-deleted documents, the tags and links they
leave orphaned, and old queue messages, then reclaim disk space.

## Usage

```
llmd vacuum [--dry-run] [--older-than <age>]
```

## Flags

| Flag | Description |
|------|-------------|
| `-n`, `--dry-run` | Report what would be purged without changing anything |
| `--older-than <age>` | Only purge items deleted more than this long ago (`30d`, `12h`) or before an RFC 3339 time |

## Rules

- An author is required, like every other command that changes the
  store: `--author` for agents and scripts, or the configured author at
  an interactive terminal. Over HTTP,
  vacuum is a POST request.
- This cannot be undone. Documents soft-deleted with `llmd rm` are
  removed with their full version history and can no longer be
  restored. Preview first with `--dry-run`.
- A soft-deleted document keeps its tags and links so that `llmd
  restore` brings it back intact. Vacuum removes them only when it
  purges the document. It also removes tags and links that were
  themselves removed with `llmd tag -d` or `llmd unlink`.
- `--older-than` keeps recent deletions restorable and purges the
  rest.
- Queue messages older than the `queue.retention` setting are removed
  in the same pass, acknowledged or not. The default is 30 days;
  `llmd config queue.retention 0` keeps every message. `--older-than`
  does not apply to them. Nothing is done when there is no work.db.
- The output lists the document paths purged, or `Would vacuum` them
  with `--dry-run`, or `Nothing to vacuum`.

## Examples

```bash
# See what a vacuum would remove
llmd --author alice vacuum --dry-run

# Purge only deletions older than thirty days, keeping recent ones restorable
llmd --author alice vacuum --older-than 30d

# Purge everything that has been deleted
llmd --author alice vacuum
```

## Notes

Vacuum also reclaims disk space in the database files. See `guide rm`
and `guide restore` for the soft-delete cycle it ends, and `guide
queue` for why the queue needs trimming.
