# llmd vacuum

Permanently delete soft-deleted documents.

## Usage

```bash
llmd vacuum
```

## Flags

| Flag | Description |
|------|-------------|
| `--older-than` | Only purge deletions older than duration |
| `-p, --path` | Only purge specific path prefix |
| `-n, --dry-run` | Show what would be deleted |
| `--force` | Skip confirmation |

## Examples

```bash
# Dry run first
llmd vacuum -n

# Permanently delete all
llmd vacuum --force

# Only old deletions
llmd vacuum --older-than 30d --force

# Only specific path
llmd vacuum -p docs/old --force
```

## Duration Format

- `7d` - 7 days
- `4w` - 4 weeks
- `3m` - 3 months

## Notes

- **Irreversible** - permanently removes data
- Requires `--force` flag or interactive confirmation
- Affects soft-deleted documents only
- Use `-n` to preview before running
