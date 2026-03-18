# LLM integration guide

How to use llmd from an AI agent. llmd is a versioned document store —
documents have paths, full version history, tags, and links. All content
is plain text.

## Self-help

Call the "guide" tool for detailed help on any topic:

  guide                        overview and all commands
  guide <topic>                detailed help (cat, grep, edit, tag, link,
                               workflow, import, export, config, mcp, ...)

## Connecting

### MCP server (recommended)

Start with `llmd mcp`. Tools are exposed over stdio using the Model
Context Protocol. Three tool names are renamed to avoid collisions:

  grep → llmd_grep    find → llmd_find    glob → llmd_glob

All other tool names match their CLI command name (cat, write, edit, ls,
rm, mv, tag, link, history, diff, restore, revert, sed, unlink, audit,
task, queue).

All tools accept: `{"args": [...], "content": "...", "author": "..."}`
Use `content` for document bodies (write, edit). Use `args` for
everything else.

### HTTP API

Start with `llmd serve`. Read commands are `GET /<command>/<path>`,
mutation commands are `POST /<command>/<path>`. Use the `Author` header
for mutations, `Output: json` for structured responses. Query parameters
map to flags (`?version=3`, `?n=5`). See `guide serve` for the full
route reference.

### CLI

Shell out to the llmd binary. Read commands work without `--author`;
mutation commands require it:

```
llmd cat notes/readme
llmd --author "Claude" write notes/readme
llmd --author "Claude" edit notes/readme "old text" "new text"
```

Use `--json` for structured output that is easier to parse.

## Author attribution

IMPORTANT: You MUST identify yourself on every mutation (write, edit,
sed, rm, mv, restore, revert, tag, unlink, link, task, audit). This is
how llmd tracks who made each change. Calls without author will be
rejected.

Via MCP tools — include "author" in every tool call:
  `{"author": "Claude", "args": ["notes/summary"], "content": "..."}`

Via CLI — pass --author on the command line:
  `llmd --author "Claude" write notes/summary`

Via HTTP — send the `Author` header on POST requests.

The "config author" setting is for the human user only — do not rely
on it.

## Commands

READ     cat <path>                  read document
         cat -n <path>               with line numbers
         cat --version N <path>      specific version
         ls [path]                   list documents

WRITE    write <path>                create/update (body via content/stdin)
         edit <path> <old> <new>     search and replace
         sed 's/old/new/' <path>     sed-style substitution

SEARCH   grep <pattern> [path]       full-text search
         find <query> [path]         search (paths only)
         glob "docs/*.md"            path pattern match

ORGANISE tag <path> <name>           add tag
         tag -f <name>               find docs by tag
         link <from> <to>            link documents
         link <path>                 list links

HISTORY  history <path>              version log
         diff <path>                 diff against previous
         diff <path>:1 <path>:2      compare two versions
         revert <path> <version>     restore old version

DELETE   rm <path>                   soft-delete
         restore <path>              recover deleted

TASKS    task list                   board view (all columns)
         task show <id>              metadata + spec body
         task add <title>            create task (body via content/stdin)
         task move <id> <column>     move to column
         task set <id> --flag hold   set flag, priority, assign, column
         task finish <id>            approve and mark done
         task rm <id>                soft-delete task (doc untouched)

AUDITS   audit add <target> [text]   create review on doc or task
         audit reply <id> [text]     reply to a thread
         audit resolve <id>          mark as approved
         audit list [target]         list audits (filterable)
         audit show <id>             full thread
         audit status                inbox: what needs my response
         audit rm <id>               soft-delete
         audit restore <id>          recover deleted audit

QUEUE    queue ls                    pending messages, oldest first
         queue peek                  next unacknowledged message
         queue ack <key>             acknowledge oldest pending
         queue send <text>           send a message (--assign for directed)
         queue history               all messages including acknowledged

VIEWS    status                      dashboard: recent docs, board, activity
         review                      pending tasks with spec previews
         review --column <name>      filter to a specific column

## Task workflow

Tasks flow through columns on a board:

  backlog → up-next → in-progress → review → done

Each task has a backing spec document that describes the work. Tasks
cannot leave the backlog until the spec has content beyond the title
(spec gating).

### Orientation

Start here to understand what needs doing:

```
status                          overview of board and recent activity
review                          all tasks with spec previews
review --column up-next         what's ready to start
review --column review          what's waiting for review
task show <id>                  full spec and metadata for one task
```

### Coder workflow

Pick up work, do it, submit for review:

```
review --column up-next         see what's ready
task show <id>                  read the spec
task move <id> in-progress      claim the work
(do the work)
task move <id> review           submit for review
```

Check your audit inbox regularly — reviewers leave feedback there:

```
audit status                    threads assigned to you
audit show <id>                 read full thread
audit reply <id> "Fixed."       respond to feedback
```

### Reviewer workflow

Review submitted work, give feedback or approve:

```
review --column review          see what's waiting
task show <id>                  read the spec
(inspect the work against the spec)
```

If the work needs changes, create an audit thread:

```
audit add <id> "Issue description" --assign <coder>
```

If the work is good, finish the task:

```
task finish <id>
```

### Audit threads

Audits are the feedback mechanism between agents. They are immutable,
insert-only review threads attached to a document or task.

- `audit add <target> "comment"` — open a thread
- `audit reply <id> "response"` — respond
- `audit resolve <id>` — mark as approved
- `audit status` — your inbox (threads assigned to you, awaiting response)

Use `--assign` to direct feedback to a specific agent. Use `--status`
to set thread status (pending, needs-work, approved, rejected, info).

See `guide audit` for full details.

## Polling for changes

Check the message queue for pending events and direct messages:

```
queue ls                          all pending messages, oldest first
queue peek                        next message to process
queue ack <key>                   acknowledge after processing
```

The queue collects all domain events (document writes, task moves, audit
creation) and direct messages from humans or agents. Poll it to discover
what happened since your last check. Process messages in order, ack each
one. See `guide queue` for details.

For finer-grained polling, use `--since` on domain commands:

```
ls --since 5m                     new/updated documents
task list --since 5m              recently created tasks
audit list --since 5m             recent audits
audit status --since 5m           recent inbox items
```

## More help

- `guide` — full command reference
- `guide workflow` — best practices and task lifecycle
- `guide task` — task board details
- `guide audit` — audit thread details
- `guide queue` — message queue for coordination
- `<command> --help` — usage for a specific command
