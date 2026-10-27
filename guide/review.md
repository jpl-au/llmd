# llmd review

List the tasks that are waiting for someone, each with its spec
preview and linked documents, so you can pick up work or check
progress.

## Usage

```
llmd review [--column name] [-n limit] [--all]
llmd review <column>                        Same as --column
```

## Flags

| Flag | Description |
|------|-------------|
| `--column <name>` | Only this column (e.g. `in-progress`) |
| `-n <limit>` | Maximum tasks to show |
| `--all` | Include done tasks and tasks flagged `human` |

## Rules

- Read-only. No author is needed.
- Without a column, done tasks and tasks flagged `human` are left out,
  and a final line counts them: `(2 done, 1 human not shown; use
  --all)`. Asking for a column, or passing `--all`, shows everything.
- `--column` wins when a column is also given as an argument. `-n`
  applies after the filtering above.
- Piped, each task is one line of seven tab-separated fields: `key
  title column P<priority> assignee flags spec`, where spec is the
  document path when the spec document exists and empty otherwise. The
  same line shape as `task list`.
- At a terminal each task is a card: key, title and column, then
  priority, assignee and flags, then the spec preview and the links.
- `--json` returns, per task, the task, its spec preview and its links.
- Spec previews skip the title heading and leading blank lines. A
  task without a spec document shows metadata only.

## Examples

```bash
# Review what is pending
llmd review

# Just what's in progress
llmd review in-progress
llmd review --column in-progress

# Top 3 tasks only
llmd review -n 3

# Everything, done and human tasks included
llmd review --all

# Machine-readable
llmd review --json
```

## Next steps

After reviewing a task:

- Read the full spec with `llmd task show <id>`.
- Flag an issue with `llmd audit add <id> "description"` to open a
  review thread. Use `--assign` to direct it to the coder.
- Approve the work with `llmd task finish <id>` to move it to done.
- Check for existing feedback with `llmd audit list <id>`.

See `guide workflow` for the full task lifecycle and review process,
and `guide task` for the board itself.
