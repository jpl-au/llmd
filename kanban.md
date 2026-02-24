# Kanban Task System — Design Document

## Problem

LLMs lose context when the context window clears. Specs and task state
need to persist across sessions. llmd already stores documents with
versioning — adding structured task management lets specs live alongside
the tasks they describe, queryable by both humans and LLMs.

## Core Concepts

### Task

A task has:
- A **title** — short description
- A **body** — the full spec, context, acceptance criteria. This is a
  document in the store, with full versioning, history, and search.
- A **status** — which column the task is in
- A **priority** — integer (1 = highest)
- A **position** — integer, defines ordering within a column
- An **assigned to** — who is working on it
- **Flags** — optional markers: `blocked`, `hold`

A task's body lives at a document path. By default, tasks are stored
under `tasks/` with a slug derived from the title (e.g.
`tasks/fix-auth-bug`). A custom path can be specified.

### Columns

Columns define the workflow. Tasks move through them left to right.

Default columns: `backlog`, `up-next`, `in-progress`, `review`, `done`.

- **Backlog** — captured, maybe specced, not committed to yet. New
  tasks land here by default.
- **Up Next** — specced, committed, this is what gets picked up next.
- **In Progress** — actively being worked on.
- **Review** — work done, needs verification.
- **Done** — complete.

### Flags

Flags are metadata on a task, not columns. A flagged task stays in
its current column.

- **blocked** — can't proceed, something is in the way.
- **hold** — paused deliberately, waiting on something external.

Flags are set and cleared via `task set`:

```
llmd task set <id> --flag blocked
llmd task set <id> --unflag blocked
```

### Spec Gating

A task without a spec (document has only the template heading or no
document at all) cannot leave the backlog column. The board view
makes this obvious — the SPEC column is empty for unspecced tasks.

This forces specs to be written before work starts. The whole point
of the system is that specs live alongside tasks.

### Relationship Between Tasks and Documents

Every task has a document. The document IS the task body. This means:

- `llmd cat tasks/fix-auth-bug` reads the spec
- `llmd edit tasks/fix-auth-bug "old" "new"` updates the spec
- `llmd history tasks/fix-auth-bug` shows spec evolution
- `llmd grep "auth" tasks/` finds tasks mentioning auth
- `llmd diff tasks/fix-auth-bug` shows what changed

The task system adds workflow metadata on top: status, priority,
position, assigned to, flags. These live in the `tasks` table, not
in the document content. The document stays clean — it's the spec,
not a mix of spec and metadata.

Tasks can also **link** to other documents outside `tasks/`:

```
llmd task link <task-id> specs/auth-design
```

This creates a standard llmd link between the task document and the
referenced doc.

## Data Model

### Two Personas

llmd serves two use cases:

1. **Document repo** — just docs. No task tables, no history tables.
   Clean, lightweight, publishable. "I keep my integration docs here."
2. **Development platform** — docs + tasks + history. "I manage specs
   and work through them."

Tables are created lazily. A fresh store has only `content` and
`entities`. The `tasks` table appears on first `llmd task add`. The
`history` table appears when something first needs to log an event.
A store that never uses tasks never has task tables.

### Column Configuration

Stored as an entity:
- Namespace: `kanban:board`
- Relation: null
- Value: `{"columns": ["backlog", "up-next", "in-progress", "review", "done"]}`

The columns array defines both the available statuses and their
left-to-right ordering. Created on first use (first `task add`
or `task columns` command).

### Tasks Table

Created on first `llmd task add`:

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    assigned_to TEXT,
    flags TEXT,
    path TEXT NOT NULL,
    author TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_tasks_key ON tasks(key);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_path ON tasks(path);
CREATE INDEX IF NOT EXISTS idx_tasks_deleted ON tasks(deleted_at) WHERE deleted_at IS NOT NULL;
```

Native columns for all task fields. Mutable — position, priority,
assigned_to, status, flags are updated in place. The `path` column
links to the document in the `content` table. The `key` column is
the short stable ID used in all CLI commands (e.g. `a1b2`).

The `flags` column stores a comma-separated list (e.g. `"blocked"`,
`"hold"`, `"blocked,hold"`). Null when no flags are set.

Follows the same conventions as other tables: `key`, `author`,
`source`, `created_at`, `deleted_at` for soft-delete.

### Task Ordering

Tasks within a column are ordered by `position` ascending, then by
`created_at` as a tiebreaker. When a new task is created, it gets
`position = max(position) + 1` for its column (appended to the
bottom).

When a task is moved to a different column, it gets the next
available position in the target column. When a task is explicitly
repositioned within its column (`task set <id> --position N`), the
other tasks in that column are renumbered.

This supports both CLI reordering now and drag-and-drop reordering
when the web UI arrives later.

### History Table

General-purpose audit log — not task-specific. Created when
something first needs to log an event:

```sql
CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    subject TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT
);

CREATE INDEX IF NOT EXISTS idx_history_subject ON history(subject);
CREATE INDEX IF NOT EXISTS idx_history_action ON history(action);
CREATE INDEX IF NOT EXISTS idx_history_timestamp ON history(timestamp);
```

Every task state change — move, priority change, reassignment,
reposition, flag change, delete, restore — writes a row here. The
`subject` is the task key, `action` is the operation (e.g. "move",
"set", "flag", "delete"), and old/new values capture what changed.
The `actor` records who made the change (human or LLM).

This table is extensible. Future features beyond tasks can log
events here too — it is not coupled to the tasks table.

The `content` table is already self-auditing (insert-only with
author), so document changes do not need the history table.

### Publishing and Vacuum

When publishing a store as a document repo:
- `vacuum` cleans soft-deleted rows from all tables
- Task and history tables can be dropped entirely if the store
  is being published as docs-only
- A store that was never used for tasks has no tables to drop

## CLI Commands

### Task Management

```
llmd task add <title>               Create a task (appended to backlog)
llmd task add <title> --status up-next
llmd task add <title> --priority 1
llmd task add <title> --assign alice
llmd task add <title> --path specs/custom-path
```

Creating a task does two things:
1. Creates a document at `tasks/<slug>` (or `--path`)
2. Inserts a row in the `tasks` table pointing at that document

The task body comes from stdin, same as `llmd write`:

```
echo "## Acceptance Criteria\n\n- Auth tokens expire after 1h" | llmd task add "Fix auth bug"

# Or with a heredoc for longer specs:
llmd task add "Fix auth bug" <<'SPEC'
## Context

The auth tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
- Refresh endpoint issues new tokens
SPEC
```

If no stdin is provided, the document is created with a minimal
template (just the title as a heading). The spec can be fleshed out
later with `llmd edit` or `llmd write`.

### Viewing Tasks

```
llmd task list                      List all active tasks (board view)
llmd task list --status in-progress Filter by column
llmd task list --assign alice       Filter by assigned to
llmd task list --priority 1         Filter by priority
llmd task show <id>                 Show task metadata + document body
```

`llmd task list` outputs a board view. Each column is a heading
followed by a markdown table of tasks, ordered by position:

```
BACKLOG

| ID    | TITLE              | PRIORITY | ASSIGNED TO | FLAGS | SPEC                   |
|-------|--------------------|----------|-------------|-------|------------------------|
| #a1b2 | Fix auth bug       | 1        | alice       |       | tasks/fix-auth-bug     |
| #c3d4 | Update API docs    | 3        |             |       |                        |
| #e5f6 | Refactor search    | 2        |             | hold  |                        |

UP NEXT

| ID    | TITLE              | PRIORITY | ASSIGNED TO | FLAGS | SPEC                   |
|-------|--------------------|----------|-------------|-------|------------------------|
| #g7h8 | Add export command | 1        | bob         |       | tasks/add-export-cmd   |

IN PROGRESS

| ID    | TITLE              | PRIORITY | ASSIGNED TO | FLAGS   | SPEC                   |
|-------|--------------------|----------|-------------|---------|------------------------|
| #i9j0 | Migrate database   | 2        | alice       | blocked | tasks/migrate-database |

REVIEW

(empty)

DONE

| ID    | TITLE              | PRIORITY | ASSIGNED TO | FLAGS | SPEC                   |
|-------|--------------------|----------|-------------|-------|------------------------|
| #k1l2 | Fix login redirect | 2        | alice       |       | tasks/fix-login-redirect |
```

Terminal output is rendered through glamour for readable, formatted
tables. Piped output and `--json` give raw structured data for LLMs
and scripts.

Task IDs are the `key` column (short, unique, stable). These are
used in all commands that reference a specific task.

`llmd task show <id>` prints the task metadata followed by the full
document body.

### Moving Tasks

```
llmd task move <id> <status>        Move task to a column
llmd task move a1b2 in-progress     Start working on it
llmd task move a1b2 done            Mark complete
```

Moving a task updates its `status` and `position` in the `tasks`
table and writes a row to the `history` table recording what changed,
when, and who did it.

### Editing Task Metadata

```
llmd task set <id> --priority 1
llmd task set <id> --assign bob
llmd task set <id> --title "New title"
llmd task set <id> --position 0          Move task to top of its column
llmd task set <id> --flag blocked
llmd task set <id> --unflag blocked
```

To edit the task body (the spec), use standard document commands:

```
llmd edit tasks/fix-auth-bug "old text" "new text"
llmd cat tasks/fix-auth-bug
```

### Archiving / Deleting Tasks

```
llmd task rm <id>                   Soft-delete the task
llmd task restore <id>              Restore a soft-deleted task
```

This sets `deleted_at` on the task row. The underlying document is
never touched. The output reminds the user:

```
Removed task #a1b2 "Fix auth bug"
Note: the document at tasks/fix-auth-bug still exists. To remove it: llmd rm tasks/fix-auth-bug
```

### Column Management

```
llmd task columns                           List columns in order
llmd task add-column <name>                 Add a column at the end
llmd task add-column <name> --after <col>   Insert after a specific column
llmd task rm-column <name>                  Remove a column (fails if tasks exist in it)
llmd task mv-column <name> --after <col>    Reorder a column
```

### Linking Tasks to Documents

```
llmd task link <id> <doc-path>      Link task to a document
llmd task links <id>                List linked documents
```

Uses standard llmd links under the hood.

## MCP Tools

All task commands exposed via MCP:

| CLI | MCP tool |
|-----|----------|
| `task add` | `task_add` |
| `task list` | `task_list` |
| `task show` | `task_show` |
| `task move` | `task_move` |
| `task set` | `task_set` |
| `task rm` | `task_rm` |
| `task restore` | `task_restore` |

This lets LLMs:
1. Read the board to understand current work
2. Pick up tasks (`task move <id> in-progress`)
3. Read the spec (`cat` the task document)
4. Update the spec as they work
5. Move to done when finished
6. Create new tasks as they discover work

## Rendering

Terminal output uses glamour (already a dependency for guides) to
render the markdown tables with proper alignment and borders. This
gives us formatted tables for free.

Piped output emits raw markdown — parseable by LLMs and scripts.
`--json` emits structured JSON for programmatic consumption.

Future: column headers and flag values can be colour-coded using
lipgloss (already a transitive dependency via glamour). Blocked in
red, hold in yellow, done in green. This is a visual enhancement,
not a blocker for the initial implementation.

## Implementation Packages

- `internal/llmd/tasks/` — task operations against the tasks table
- `internal/llmd/history/` — general-purpose audit log
- `cli/task.go` — task CLI commands
- `sdk/sdk.go` — add task methods to the SDK
- `internal/host/api.go` — bridge SDK to tasks package
- `guide/task.md` — documentation

## Design Decisions

1. **`task add` without stdin** creates a minimal document with the
   title as a heading. The spec is filled in later via `llmd edit`
   or `llmd write`. Tasks without a spec cannot leave backlog — spec
   gating enforces "write the spec before starting work."

2. **One board per store.** The store is the project. No multi-board
   support needed.

3. **No WIP limits.** Primary user is an LLM working tasks
   sequentially. No artificial constraints.

4. **No due dates or start dates.** The history table captures when
   tasks move between columns — timestamps come for free from the
   audit log.

5. **`task rm` is task-only.** Soft-deletes the task row. The
   underlying document is always untouched. Output includes a hint
   showing how to remove the document if desired. No `--hard` flag
   — never accidentally delete a spec.

6. **Lazy table creation.** The `tasks` and `history` tables are
   created on first use, not at init. A doc-only store never has
   them. This keeps the document repo persona clean and lightweight.

7. **Mutable tasks, audited history.** The `tasks` table is mutable
   (current state, fast reads). Every change writes to the `history`
   table (who, what, when, old value, new value). Full observability
   without the complexity of insert-only on the tasks table itself.

8. **Flags, not columns, for exceptional states.** Blocked and hold
   are flags on a task, not separate columns. A blocked task stays
   in its current column with a `blocked` flag. This preserves
   where the task was in the workflow when it got stuck.

9. **Markdown table output.** The board view is markdown tables
   rendered through glamour for terminals. This gives formatted
   output for humans and structured output for LLMs with no extra
   work.
