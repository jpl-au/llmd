# llmd status

Store overview dashboard. Shows recent documents, task board summary,
and recent activity in a single screen.

## Usage

```
llmd status [-n limit]
```

## Flags

| Flag | Description |
|------|-------------|
| `-n N` | Items per section (default 5) |

## Sections

**Recent documents** — the most recently modified documents, showing
path, version, author, and date.

**Task board** — task counts per column at a glance.

**Recent activity** — the latest task events from the audit log, showing
what changed and when.

## Examples

```bash
# Quick overview
llmd status

# Show more history
llmd status -n 10

# Machine-readable
llmd status --json
```

## Notes

- Output is styled with tables and colour when run in a terminal.
- Piped output is plain text, suitable for scripts and LLMs.
- Combine with `llmd review` for deeper task context.
- Use `llmd audit status` to check your audit inbox — threads assigned
  to you that need a response.
- See `guide workflow` for the full task lifecycle.
