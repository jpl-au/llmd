# llmd sync

Sync filesystem changes back to database.

## Usage

```bash
llmd sync
```

## Flags

| Flag | Description |
|------|-------------|
| `-n, --dry-run` | Show what would be synced |

## Examples

```bash
# Preview changes
llmd sync -n

# Sync changes
llmd sync
```

## When to Use

Documents are mirrored to `.llmd/` when `sync.files` is enabled. If someone edits these files directly (bypassing llmd), use `llmd sync` to import those changes back.

## Notes

- The database is the source of truth
- `llmd sync` is a recovery mechanism, not the normal workflow
- Normal workflow: use `llmd write` and `llmd edit`
- LLMs should always use `-a` flag to identify themselves
