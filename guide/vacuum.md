# llmd vacuum

Permanently remove soft-deleted documents and the tags and links they
leave orphaned, then reclaim disk space.

## Usage

```
llmd vacuum [--dry-run] [--older-than <age>]
```

| Flag | Description |
|------|-------------|
| `-n`, `--dry-run` | Report what would be purged without changing anything |
| `--older-than` | Only purge items deleted more than this long ago (`30d`, `12h`) or before an RFC 3339 time |

## Examples

```bash
# See what a vacuum would remove
llmd vacuum --dry-run

# Purge only deletions older than thirty days, keeping recent ones restorable
llmd vacuum --older-than 30d

# Purge everything that has been deleted
llmd vacuum
```

## Notes

- Permanently deletes documents that were soft-deleted with `llmd rm`,
  including their full version history. They can no longer be restored.
- A soft-deleted document keeps its tags and links so that `llmd restore`
  brings it back intact. Vacuum removes them only when it purges the
  document.
- Also removes tags and links that were themselves removed with
  `llmd tag -d` or `llmd unlink`.
- Reclaims disk space in the database files.
- `--dry-run` lists the document paths that would be purged. Without it,
  the same list shows what was purged.
