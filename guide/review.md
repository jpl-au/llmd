# llmd review

Review pending tasks with inline context. Shows task metadata, spec
document previews, and linked documents — everything you need to pick
up work or check on progress.

## Usage

```
llmd review [--column name] [-n limit]
```

## Flags

| Flag | Description |
|------|-------------|
| `--column <name>` | Filter by column (e.g. `in-progress`) |
| `-n N` | Maximum tasks to show |

## Examples

```bash
# Review all tasks
llmd review

# Just what's in progress
llmd review in-progress
llmd review --column in-progress

# Top 3 tasks only
llmd review -n 3

# Machine-readable
llmd review --json
```

## Output

For each task, the review shows:

- **Header** — task key, title, and column
- **Metadata** — priority, assigned to, flags
- **Spec preview** — first few lines of the task's spec document
- **Links** — documents linked to the task

## Notes

- Output is styled when run in a terminal. Piped output is plain
  tab-separated text, suitable for scripts and LLMs.
- Spec previews skip the title heading and leading blank lines.
- Tasks without a spec document show metadata only.
- Use `llmd task show <id>` for the full spec body.
