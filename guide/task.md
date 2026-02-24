# llmd task

Manage tasks on the board. Tasks track work through columns (backlog,
up-next, in-progress, review, done). Each task has a backing document
that holds the spec.

## Usage

```
llmd task add <title>                Create a task
llmd task list                       Board view (all columns)
llmd task show <id>                  Show task metadata + spec
llmd task move <id> <status>         Move task to a column
llmd task set <id> [flags]           Update task metadata
llmd task rm <id>                    Soft-delete task
llmd task restore <id>               Restore deleted task
llmd task columns                    List columns
llmd task add-column <name>          Add a column
llmd task rm-column <name>           Remove empty column
llmd task mv-column <name> --after   Reorder column
llmd task link <id> <path>           Link task to document
llmd task links <id>                 List linked documents
```

## Add flags

| Flag | Description |
|------|-------------|
| `--status <col>` | Create in a specific column (default: backlog) |
| `--priority <n>` | Set priority (1 = highest) |
| `--assign <name>` | Assign to someone |
| `--path <path>` | Custom document path (default: tasks/\<slug\>) |

The task body comes from stdin, same as `write`:

```bash
echo "## Acceptance Criteria" | llmd task add "Fix auth bug"
```

If no stdin is provided, the document is created with a minimal
template heading. Tasks without a real spec cannot leave backlog.

## Set flags

| Flag | Description |
|------|-------------|
| `--title <text>` | Change the title |
| `--priority <n>` | Change priority |
| `--assign <name>` | Reassign |
| `--position <n>` | Reorder within column |
| `--flag <name>` | Set a flag (blocked, hold) |
| `--unflag <name>` | Remove a flag |

## List flags

| Flag | Description |
|------|-------------|
| `--status <col>` | Filter by column |
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
llmd task add-column testing --after in-progress
```

## Notes

- Task IDs are 9-character keys, shown as `#a1b2c3d4e` in output.
- `task rm` only deletes the task row. The document at the task's path
  is never touched. Use `llmd rm <path>` separately if needed.
- Spec gating: tasks with no real document content cannot be moved out
  of backlog. Write the spec first.
- Flags (`blocked`, `hold`) are metadata on the task, not columns. A
  flagged task stays in its current column.
- All state changes are recorded in the history table for observability.
- The board view shows markdown tables grouped by column.
- Default columns: backlog, up-next, in-progress, review, done.
  Customise with `add-column`, `rm-column`, `mv-column`.
