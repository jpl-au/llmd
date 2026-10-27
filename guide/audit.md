# llmd audit

Review threads on tasks and documents, between agents and people,
kept as insert-only records.

## Usage

```
llmd audit add <target> [content]    Create a top-level audit
llmd audit reply <id> [content]      Reply to an existing thread
llmd audit list [target]             List audits (optionally filtered)
llmd audit show <id>                 Display full audit with thread
llmd audit resolve <id>              Mark as approved
llmd audit rm <id>                   Soft-delete
llmd audit restore <id>              Recover a soft-deleted audit
llmd audit status                    Inbox: what needs my response
```

`<target>` is a task ID or a document path. `<id>` is an audit ID.

## Flags

| Flag | Subcommands | Description |
|------|-------------|-------------|
| `--status <s>`, `-s` | add, reply, list | Set or filter by status |
| `--assign <name>` | add, reply, list | Assign to or filter by assignee |
| `--file <path>` | add, reply | Read content from a filesystem path |
| `--version <n>` | add | Pin to a specific document version |
| `--pending` | list | Filter to pending/needs-work |
| `--by-author <name>` | list | Filter by who created the audit |
| `--since <time>` | list, status | Only audits created after a time (e.g. `5m`, `1h`, RFC 3339) |
| `--all` | show | Show the full thread, no cap |

## Rules

- `add`, `reply`, `resolve`, `rm`, `restore` and `status` need an
  author. `list` and `show` need none.
- Content for `add` and `reply` is resolved in order: positional
  argument, then `--file`, then stdin. `resolve` needs no content.
- Nothing is ever updated. A thread's status and assignee are those of
  its most recent entry. A reply with `--status approved` both responds
  and resolves the thread.
- Threads are single-level. Replying to a reply attaches to the
  top-level parent.
- A 9-character base36 key as the target means a task; anything else
  is a document path.
- `show` prints the root message and the 9 most recent replies (10
  entries). `--all` shows the whole thread.
- `status` lists threads assigned to the author whose status is
  `pending` or `needs-work`.
- IDs are bare 9-character base36 keys with no prefix. `--json` gives
  the structured form.

## Examples

```bash
# Inline content
llmd --author "gemini" audit add docs/api "Error handling is incomplete."

# Content from stdin
echo "Detailed review..." | llmd --author "gemini" audit add docs/api

# Content from file
llmd --author "gemini" audit add docs/api --file review.md

# With status and assignee
llmd --author "gemini" audit add docs/api "Needs work." \
  --status needs-work --assign claude-code

# Simple reply
llmd --author "claude-code" audit reply 0mmsfn7h1 "Fixed."

# Reply and resolve in one action
llmd --author "claude-code" audit reply 0mmsfn7h1 --status approved

# Reassign to another agent
llmd --author "claude-code" audit reply 0mmsfn7h1 "Done, please check." \
  --assign gemini

# Quick resolve (inserts "approved" entry, no content needed)
llmd --author "claude-code" audit resolve 0mmsfn7h1

# All audits
llmd audit list

# Audits on a specific document
llmd audit list docs/api

# Only pending/needs-work
llmd audit list --pending

# By creator
llmd audit list --by-author gemini

# By assignee
llmd audit list --assign claude-code

# By exact status
llmd audit list --status needs-work

# Only audits from the last hour
llmd audit list --since 1h

# Default: root message + the 9 most recent replies (up to 10 total)
llmd audit show 0mmsfn7h1

# Full thread, no cap
llmd audit show --all 0mmsfn7h1

# Soft-delete, then recover
llmd --author "claude-code" audit rm 0mmsfn7h1
llmd --author "claude-code" audit restore 0mmsfn7h1

# What needs my attention?
llmd --author "claude-code" audit status

# Uses config author in interactive terminals
llmd audit status
```

## How threading works

- All threads are single-level. Replying to a reply resolves to the
  top-level parent automatically.
- Thread status is the status of the most recent entry. A reply with
  `--status approved` both responds and resolves the thread.
- Assignee propagates through replies. If a reply omits `--assign`,
  it inherits from the parent.

Long audit threads are common on multi-round reviews. The default
`show` prints the root message plus the most recent replies so agents
see both "what was being reviewed" and "what's the current state"
without having every middle message dumped into their context. Use
`--all` to see the full thread.

## How status works

`audit status` shows threads that need your attention:

- The thread is assigned to you (effective assignee), AND
- The effective status is `pending` or `needs-work`

Replying to a thread that stays assigned to you keeps it in your inbox;
reassign it with `--assign` or resolve it to clear it.

Status values are free-form strings. Common conventions:
`pending`, `approved`, `needs-work`, `rejected`, `info`.

## Task review workflow

Audits are the primary feedback mechanism for task review. When a task
lands in the review column:

1. Reviewer reads the spec: `task show <id>`
2. Reviewer inspects the work against the spec.
3. If changes are needed: `audit add <id> "description" --assign <coder>`
4. Coder checks inbox: `audit status`
5. Coder fixes and responds: `audit reply <audit-id> "Fixed."`
6. Reviewer resolves the thread: `audit resolve <audit-id>`
7. Once all threads are clear: `task finish <id>`

See `guide workflow` for the full task lifecycle and `guide task` for
task board details.
