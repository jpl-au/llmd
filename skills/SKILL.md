---
name: llmd
description: >
  Use llmd to manage versioned documents, track tasks on a kanban board, run
  audit review threads, search content, and orchestrate AI agents. Trigger when
  the user wants to create/read/edit/search documents, manage tasks or the task
  board, run audits or reviews, use the message queue, or coordinate multi-agent
  workflows.
allowed-tools:
  - mcp__llmd__cat
  - mcp__llmd__write
  - mcp__llmd__edit
  - mcp__llmd__sed
  - mcp__llmd__rm
  - mcp__llmd__mv
  - mcp__llmd__restore
  - mcp__llmd__revert
  - mcp__llmd__ls
  - mcp__llmd__llmd_grep
  - mcp__llmd__llmd_find
  - mcp__llmd__llmd_glob
  - mcp__llmd__history
  - mcp__llmd__diff
  - mcp__llmd__tag
  - mcp__llmd__link
  - mcp__llmd__unlink
  - mcp__llmd__task
  - mcp__llmd__audit
  - mcp__llmd__queue
  - mcp__llmd__agent
  - mcp__llmd__rule
  - mcp__llmd__guide
  - mcp__llmd__llmd_init
---

# llmd - versioned document store and task board

llmd is a local, SQLite-backed document store with versioning, full-text search,
a kanban task board, audit review threads, and AI agent orchestration. All tools
are exposed as MCP tools.

## Tool calling convention

Every MCP tool accepts the same three parameters:

```json
{
  "args": ["subcommand", "arg1", "--flag", "value"],
  "content": "document body text (for write/edit)",
  "author": "your-name"
}
```

- **args** - positional arguments and flags, as a string array
- **content** - document body for write operations (write, edit, task add, audit add/reply)
- **author** - required on every mutation; use the user's name or "Claude"

Three tool names are prefixed to avoid collisions with built-in tools:
`llmd_grep`, `llmd_find`, `llmd_glob`. All others match their CLI command name.

## Self-help

Call `mcp__llmd__guide` for detailed help on any topic:

```
guide                          overview and all commands
guide <topic>                  detailed help (workflow, task, audit, agent, rule, queue, ...)
```

## Core workflows

### Documents

```
cat <path>                     read a document
cat --version N <path>         read a specific version
write <path>                   create or update (body via content param)
edit <path> <old> <new>        search and replace
sed 's/old/new/' <path>        sed-style substitution
rm <path>                      soft-delete
mv <from> <to>                 move/rename
restore <path>                 recover deleted
history <path>                 version log
diff <path>                    diff against previous version
revert <path> <version>        roll back to old version
```

### Search

```
llmd_grep <pattern> [path]     full-text search (FTS5 syntax: AND, OR, NOT, NEAR, "phrases", prefix*)
llmd_find <query> [path]       full-text search, paths only
llmd_glob <pattern>            shell-style path matching (*, **, ?)
ls [path]                      list documents (-l for details, --tree for hierarchy)
```

### Tags and links

```
tag <path> <name>              add a tag
tag -d <path> <name>           remove a tag
tag -f <name>                  find documents by tag
tag                            list all tags with counts
link <from> <to>               create a directed link
link <path>                    list outgoing links
link --in <path>               list incoming links
unlink <from> <to>             remove a link
```

### Tasks

Tasks flow through board columns: backlog, up-next, in-progress, review, approval, done, blocked.

```
task list                      board view (all columns)
task add <title>               create task (spec body via content param)
task show <id>                 metadata + spec body
task move <id> <column>        move to column
task set <id> [flags]          update metadata (--priority, --assign, --flag, --unflag)
task start <id>                start work (records git branch, moves to in-progress)
task finish <id>               approve and mark done
task rm <id>                   soft-delete
task diff <id>                 git diff for task's branch
task files <id>                list changed files
task link <id> <path>          link task to document
task column list               list columns
task column add <name>         add column
```

Tasks cannot leave backlog until their spec document has content beyond the title (spec gating).

### Audits (review threads)

Audits are immutable, insert-only threads attached to tasks or documents.

```
audit add <target> [text]      create review (body via content param or inline)
audit reply <id> [text]        reply to thread
audit resolve <id>             mark as approved
audit list [target]            list audits (--pending, --by-author, --assign, --since)
audit show <id>                full thread
audit status                   inbox: what needs my response
```

### Queue (cross-agent messages)

```
queue send <text>              send a message (--assign for directed)
queue ls                       pending messages
queue peek                     next unacknowledged message
queue ack <key>                acknowledge oldest pending
queue history                  all messages including acknowledged
```

### Agents and automation

```
agent add <name>               register (claude-code, gemini, aider, or custom)
agent ls                       list registered agents
agent spawn <task> <agent>     spawn agent for a task
agent runs                     list runs (--status, --task filters)
agent stop <task>              stop running agent
rule list                      display column rules
rule set <col> [flags]         configure automation (--agent, --role, --success, --failure)
```

## Task lifecycle example

```
task add "Fix auth tokens"     create task with spec in content param
task move <id> in-progress     claim the work
(do the work)
task move <id> review          submit for review
audit add <id> "Looks good"   leave review feedback
task finish <id>               approve and complete
```

## Tips

- Use `--json` in args for structured output when you need to parse results
- `task list` shows the full board; `task list --column review` filters by column
- `audit list --pending` shows unresolved review threads needing attention
- `guide` returns an overview of all commands; `guide <topic>` returns detailed help (workflow, task, audit, agent, rule, queue, etc.)
- Document paths use forward slashes, no leading slash: `docs/api`, `notes/meeting`
- Task IDs are 9-character base36 keys (e.g. `a1b2c3d4e`)
- FTS5 search supports: implicit AND, explicit OR/NOT, NEAR(), prefix *, "exact phrases"
- Always include `author` on mutations or the call will be rejected
