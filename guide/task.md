# llmd task

Manage tasks on the board. Tasks track work through columns (backlog,
up-next, in-progress, review, done). Each task has a backing document
that holds the spec.

## Usage

```
llmd task add <title>                Create a task
llmd task list                       Board view (all columns)
llmd task show <id>                  Show task metadata + spec
llmd task move <id> <column>         Move task to a column
llmd task set <id> [flags]           Update task metadata
llmd task rm <id>                    Soft-delete task
llmd task restore <id>               Restore deleted task
llmd task column list                List columns
llmd task column add <name>          Add a column
llmd task column rm <name>           Remove empty column
llmd task column mv <name> --after   Reorder column
llmd task link <id> <path>           Link task to document
llmd task links <id>                 List linked documents
llmd task start <id>                 Start task (record branch + in-progress)
llmd task diff <id>                  Show git diff for task's branch
llmd task files <id>                 List files changed on task's branch
```

## Add flags

| Flag | Description |
|------|-------------|
| `--column <col>` | Create in a specific column (default: backlog) |
| `--priority <n>` | Set priority (1 = highest) |
| `--assign <name>` | Assign to someone |
| `--path <path>` | Use an existing store document as the spec |
| `--file <file>` | Read spec content from a file on disk |

The task body can come from stdin, a file, or an existing document:

```bash
# Pipe content as spec
echo "## Acceptance Criteria" | llmd task add "Fix auth bug"

# Read spec from a local file
llmd task add "Fix auth bug" --file spec.md

# Link to an existing document in the store
llmd task add "Fix auth bug" --path docs/auth-spec
```

If no body is provided, the task is created without a document.

### Spec gating

Tasks cannot leave the backlog until their spec document has real
content beyond the title heading. A document containing only
`# Fix auth tokens` is not enough — add context, acceptance criteria,
or any detail that describes what the work actually is.

```bash
# This won't pass spec gating (title only):
echo "# Fix auth tokens" | llmd write tasks/fix-auth-tokens

# This will (content after the heading):
cat <<'EOF' | llmd write tasks/fix-auth-tokens
# Fix auth tokens

Tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
EOF
```

If a task has no document at all, write one with
`llmd write tasks/<slug>` or link an existing document with
`llmd task link <id> <path>`.

## Set flags

| Flag | Description |
|------|-------------|
| `--title <text>` | Change the title |
| `--priority <n>` | Change priority |
| `--assign <name>` | Reassign |
| `--position <n>` | Reorder within column |
| `--flag <name>` | Set a flag (blocked, hold) |
| `--unflag <name>` | Remove a flag |
| `--branch <name>` | Set git branch manually |

## List flags

| Flag | Description |
|------|-------------|
| `--column <col>` | Filter by column |
| `--assign <name>` | Filter by assigned to |
| `--priority <n>` | Filter by priority |

## Examples

```bash
# Create a task with a spec
llmd task add "Fix auth tokens" <<'SPEC'
## Context

Auth tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
SPEC

# View the board
llmd task list

# Start working on a task
llmd task move a1b2c3d4e in-progress

# Read the spec
llmd cat tasks/fix-auth-tokens

# Update the spec
llmd edit tasks/fix-auth-tokens "old text" "new text"

# Mark blocked
llmd task set a1b2c3d4e --flag blocked

# Unblock and move to review
llmd task set a1b2c3d4e --unflag blocked
llmd task move a1b2c3d4e review

# Mark complete
llmd task move a1b2c3d4e done

# Delete a task (document is untouched)
llmd task rm a1b2c3d4e

# Add a custom column
llmd task column add testing --after in-progress
```

## Git integration

Tasks can be linked to git branches. When you start a task, llmd
records the current branch. You can then view the diff or list changed
files without remembering which branch goes with which task.

### Start

`task start` moves a task to in-progress and records the current
git branch automatically:

```bash
git checkout -b feature-auth
llmd task start a1b2c3d4e
# Started a1b2c3d4e on branch feature-auth
```

Use `--column` to move to a different column instead of in-progress.

### Diff and files

Once a task has a branch, view what changed:

```bash
# Full diff against default branch (main or master)
llmd task diff a1b2c3d4e

# Just the stats
llmd task diff a1b2c3d4e --stat

# List changed files only
llmd task files a1b2c3d4e

# Specify a different base branch
llmd task diff a1b2c3d4e --base develop
```

### Manual branch assignment

Set or change the branch without moving the task:

```bash
llmd task set a1b2c3d4e --branch feature-auth
```

## Notes

- Task IDs are 9-character keys (e.g. `a1b2c3d4e`).
- `task rm` only deletes the task row. The document at the task's path
  is never touched. Use `llmd rm <path>` separately if needed.
- Spec gating: tasks cannot leave the backlog until their document has
  content beyond the title heading. See "Spec gating" above.
- Flags (`blocked`, `hold`) are metadata on the task, not columns. A
  flagged task stays in its current column.
- All state changes are recorded in the history table for observability.
- The board view shows styled terminal tables grouped by column.
- Default columns: backlog, up-next, in-progress, review, done.
  Customise with `task column add`, `task column rm`, `task column mv`.
