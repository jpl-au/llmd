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

## Next steps

After reviewing a task:

- **Read the full spec** — `llmd task show <id>` for complete details.
- **Flag an issue** — `llmd audit add <id> "description"` to open a
  review thread. Use `--assign` to direct it to the coder.
- **Approve the work** — `llmd task finish <id>` to move it to done.
- **Check for existing feedback** — `llmd audit list <id>` to see
  whether there are already open threads on this task.

See `guide workflow` for the full task lifecycle and review process.

## Notes

- Output is styled when run in a terminal. Piped output is plain
  tab-separated text, suitable for scripts and LLMs.
- Spec previews skip the title heading and leading blank lines.
- Tasks without a spec document show metadata only.
- Use `llmd task show <id>` for the full spec body.
