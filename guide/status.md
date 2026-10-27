# llmd status

Show the store at a glance: recent documents, task counts per column,
and the latest activity, on one screen.

## Usage

```
llmd status [--limit N]
```

## Flags

| Flag | Description |
|------|-------------|
| `-n`, `--limit <int>` | Items per section (default 5) |

## Rules

- Read-only. No author is needed.
- Three sections, each capped at `--limit` items: recent documents (path,
  version, author, date, newest first), the task board (a count per
  column) and recent activity (document writes, tags, links and task
  events, newest first).
- At a terminal the sections are styled tables. Piped, they are plain
  labelled sections: `Recent documents:`, `Task board:` and `Recent
  activity:`, one indented line per item.
- `--json` returns the same three sections as structured data.

## Examples

```bash
# Quick overview
llmd status

# Show more history
llmd status --limit 10

# Machine-readable
llmd status --json
```

## Notes

- Combine with `llmd review` for deeper task context.
- Use `llmd audit status` to check your audit inbox: threads assigned
  to you that need a response.
- See `guide workflow` for the full task lifecycle.
