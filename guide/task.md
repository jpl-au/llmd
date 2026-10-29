# llmd task

Track work on a board: tasks move through columns, each task has a
spec document, and agents claim work with `task next`.

## Usage

```
llmd task list [--column c] [--all]   Board view, or one column (ls, board: aliases)
llmd task show <id>                   Metadata and spec body
llmd task add <title>                 Create a task (spec on stdin)
llmd task next [--from c] [--to c]    Claim the first ready task
llmd task move <id> <column>          Move a task to a column
llmd task set <id> [flags]            Update task fields
llmd task start <id>                  Move to in-progress, record the git branch
llmd task start <id> --agent <name>   Spawn an agent on the task (see `guide agent`)
llmd task branch <id>                 Create a git branch, then start
llmd task finish [id]                 Move to done with a git summary
llmd task rm <id>                     Soft-delete a task
llmd task restore <id>                Restore a deleted task
llmd task link <id> <path>            Link a task to a document
llmd task links <id>                  List linked documents
llmd task log <id>                    Change history of a task
llmd task chain <id>                  Dependency chain
llmd task ready <id>                  Whether every dependency is done
llmd task diff [id]                   Git diff for the task's branch
llmd task files [id]                  Files changed on the branch
llmd task commits [id]                Commits on the branch
llmd task column list                 List columns
llmd task column add <name>           Add a column
llmd task column rm <name>            Remove an empty column
llmd task column mv <name> --after c  Reorder a column
```

Default columns: backlog, up-next, in-progress, review, approval,
done, blocked. Task IDs are 10-character keys such as `a1b2c3d4e5`.

## Flags

Flags belong to a subcommand. `add`, `set` and `list` share some names
with different meanings.

| Flag | Subcommands | Description |
|------|-------------|-------------|
| `--column <col>` | add, list, start, branch, finish | Column to create in (default backlog), filter by, or move to |
| `--priority <n>` | add, set, list | Priority: 1 is the highest, larger numbers lower, 0 none |
| `--assign <name>` | add, set, list, next, start | Assignee |
| `--agent <name>` | start | Spawn this registered agent on the task; it becomes the assignee |
| `--budget <usd>` | start | Most the spawned agent may spend |
| `--worktree` | start | Spawn the agent in its own git worktree, whatever `agent.worktree` says |
| `--no-worktree` | start | Spawn the agent in the main checkout, whatever `agent.worktree` says |
| `--depends-on <id>` | add, set | Task that must be done first; empty value clears it |
| `--path <path>` | add | Use an existing store document as the spec |
| `--file <file>` | add | Read the spec from a file on disk |
| `--title <text>` | set | Change the title |
| `--position <n>` | set | Reorder within the column |
| `--flag <name>` | set | Add a flag: blocked, hold or human |
| `--unflag <name>` | set | Remove a flag |
| `--branch <name>` | set | Set the git branch by hand |
| `--since <time>` | list | Only tasks created after (`5m`, `1h`, RFC 3339) |
| `--all` | list | List every done task |
| `--from <col>` | next | Column to take from (default up-next) |
| `--to <col>` | next | Column to move to (default in-progress) |
| `--name <branch>` | branch | Branch name (default `task/<slug>`) |
| `--after <col>` | column add, column mv | Place the column after this one |
| `--base <branch>` | diff, files, commits, finish | Base branch (default: main or master) |
| `--stat` | diff | Diffstat only |
| `-n`, `--limit <int>` | log | Most recent events to show |

## Rules

- Changing anything needs an author: `--author` for agents and
  scripts. Reads need none.
- A task cannot leave the backlog until its spec document has content
  beyond the title heading. Write the spec with `llmd write
  tasks/<slug>`, or link one with `task link`.
- Within a column, tasks with a priority come first, 1 before 2, then
  tasks without one; board position breaks ties. The board, `review`
  and `next` all use this order, so what an agent takes is the top of
  what a person sees. `llmd config task.order position` switches to
  board position alone. A negative priority is refused.
- `next` takes the first task in that order, moves it and assigns it
  in one write. Two callers never get the same task. It passes over
  tasks flagged human, hold or blocked, tasks whose dependencies are
  not done, and backlog tasks without a spec. Nothing ready is an
  error, so a loop can stop.
- Flags are not columns. A flagged task stays where it is. `human`
  means the task is for the user: agents leave it alone and `review`
  hides it, unless `--column` asks for its column.
- The board shows the newest 10 done tasks and says how many more
  there are. `--all` or `--column done` shows them all.
- `rm` deletes the task only. The spec document stays; remove it with
  `llmd rm <path>` if you mean to.
- Piped, `task list` prints `COLUMN (count)` headings with one task per
  line: `key  title  column  priority  assignee  flags  spec`, seven
  tab-separated fields. Priority reads `P1`, `P2` and so on, or `-` for
  none; spec is the document path when the document exists.
  `task show` and `task next` print raw markdown. `task log` prints
  tab-separated lines with a header line.

## Examples

```bash
# Create a task with a spec on stdin
llmd --author alice task add "Fix auth tokens" <<'SPEC'
## Context

Auth tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
SPEC

# Queue it for an agent
llmd --author alice task move a1b2c3d4e5 up-next

# An agent claims the next piece of work and gets the spec back
llmd --author claude-code task next

# ... does the work, then hands it over
llmd --author claude-code task move a1b2c3d4e5 review

# A reviewer takes the next task waiting for review
llmd --author reviewer task next --from review --to approval

# Approve and record the git summary
llmd --author reviewer task finish a1b2c3d4e5

# Keep a task for yourself: agents pass it over, review hides it by default
llmd --author alice task set a1b2c3d4e5 --flag human

# Mark blocked, then clear it
llmd --author alice task set a1b2c3d4e5 --flag blocked
llmd --author alice task set a1b2c3d4e5 --unflag blocked

# Other ways to supply a spec
llmd --author alice task add "Fix auth bug" --file spec.md
llmd --author alice task add "Fix auth bug" --path docs/auth-spec

# Add a column after in-progress
llmd --author alice task column add testing --after in-progress
```

## Specs

Every task points at a document that holds its spec, by default
`tasks/<slug>` made from the title. The spec is where the work is
described: context, acceptance criteria, links to other documents.
A task created without a body has no document yet.

Spec gating stops a task leaving the backlog until the document says
more than its title:

```bash
# Not enough: a title only
echo "# Fix auth tokens" | llmd --author alice write tasks/fix-auth-tokens

# Enough: anything after the heading
printf '# Fix auth tokens\n\nTokens never expire, causing security issues.\n' |
  llmd --author alice write tasks/fix-auth-tokens
```

Edit a spec like any document: `llmd cat tasks/fix-auth-tokens`,
`llmd edit tasks/fix-auth-tokens "old" "new"`.

## Git integration

`task start` records the current git branch on the task. `task
branch` creates one named `task/<slug>` first, or `--name` for another
name. Once a task has a branch, `task diff`, `task files` and `task
commits` show what changed against the default branch, and `task
finish` reports the file and commit counts. All take `--base` for
another base branch. Given no id, they find the task from the current
branch.

```bash
git checkout -b feature-auth
llmd --author alice task start a1b2c3d4e5     # Started a1b2c3d4e5 on branch feature-auth
llmd task diff a1b2c3d4e5 --stat
llmd --author alice task set a1b2c3d4e5 --branch feature-auth   # set the branch by hand
```

`next` records no branch: it hands a task over and leaves branching
to the worker. Follow it with `task branch <id>` to work on a branch
of its own.

Git is optional. Without it, `start` and `finish` still move the
task; `branch`, `diff`, `files` and `commits` report that git is
unavailable.

## Review and feedback

A task in review is checked with `llmd review --column review` and
discussed in audit threads:

```bash
llmd --author reviewer audit add a1b2c3d4e5 "Error handling missing" --assign coder
llmd --author coder audit status      # what needs my response
llmd --author reviewer task finish a1b2c3d4e5
```

See `guide audit` for threads and `guide workflow` for the full task
lifecycle. Columns can run agents automatically when a task enters
them: see `guide rule`.

## Notes

- Every change to a task is recorded; `task log <id>` shows it.
- Position is the hand order within a column. `set --position`
  reorders; `move` and `next` put a task at the end of its new column.
  With the default order, position only decides between tasks of equal
  priority; set `task.order` to `position` to make it the whole order.
- A dependency (`--depends-on`) is a single task. `chain` follows the
  dependencies back to the first; `ready` says whether all are done.
