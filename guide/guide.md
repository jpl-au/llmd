# llmd guide

llmd is a document store and task board for a project, kept in SQLite
under `.llmd/`, used by people at a terminal and by agents through the
same commands.

## Usage

```
llmd guide              This overview
llmd guide <topic>      One command or topic in full, e.g. guide task
llmd <command> --help   Usage, flags and a pointer to the guide
```

## Rules

- Changing anything needs an author. Agents and scripts pass
  `--author <name>` on every call that writes; a person at a terminal
  sets one once with `llmd config author`. Reads need no author. Never
  change the configured author on someone else's behalf.
- Document paths look like `notes/plan`: forward slashes, no leading
  slash, no extension. A single word is a fine path.
- Every write is a new version. `write` replaces a document, `edit`
  does search-and-replace, `sed` substitutes, `rm` soft-deletes, and
  `history`, `diff`, `revert` and `restore` undo any of it.
- Tasks move through columns: backlog, up-next, in-progress, review,
  approval, done, blocked. A task cannot leave the backlog until its
  spec document says more than its title.
- To pick up work, call `task next`: it claims the first ready task
  (priority 1 first, then board order) in one write and returns its
  spec. Then work, then `task move <id>
  review` or `task finish <id>`. Raise problems with `audit add <id>`.
  Tasks flagged `human` are for the user: leave them alone.
- Output that would grow without bound is capped and says so: `ls`
  shows 500 documents, `diff` 500 lines, `task list` the newest 10 done
  tasks, `review` leaves done and human tasks out. `--all` lifts each
  cap. `--json` gives the structured form of any command.
- Piped, output is plain: tables become tab-separated lines with a
  header line, markdown comes out raw, and tasks print as one line of
  seven tab-separated fields (`key title column P<n> assignee flags
  spec`). At a terminal the same commands render tables and colour.

## Quick start

```
llmd init                                    # create a store in .llmd/
llmd config author "Your Name"               # set your author name
echo "Hello, world." | llmd write notes/hi   # write a document
llmd cat notes/hi                            # read it back
llmd ls                                      # list documents
llmd grep hello                              # full-text search
```

## Commands

### Document operations

| Command   | Description                          | Usage                              |
|-----------|--------------------------------------|------------------------------------|
| `cat`     | Read a document                      | `cat [-n] [--version N] <path>`    |
| `write`   | Create or update a document          | `echo "..." \| write <path>`       |
| `edit`    | Search and replace within a document | `edit <path> <old> <new>`          |
| `sed`     | sed-style substitution               | `sed 's/old/new/' <path>`          |
| `rm`      | Soft-delete a document               | `rm <path>`                        |
| `mv`      | Move or rename a document            | `mv <from> <to>`                   |
| `restore` | Recover a soft-deleted document      | `restore <path>`                   |
| `revert`  | Roll back to a previous version      | `revert <path> <version>`          |

### Search and discovery

| Command | Description                            | Usage                         |
|---------|----------------------------------------|-------------------------------|
| `ls`    | List documents                         | `ls [-l] [-a] [-t] [path]`   |
| `grep`  | Full-text search (FTS5 syntax)         | `grep [-n] [-l] [-c] <query>` |
| `find`  | Full-text search, paths only           | `find <query> [path]`         |
| `glob`  | Match documents by path pattern        | `glob <pattern>`              |

### Version control

| Command   | Description                        | Usage                          |
|-----------|------------------------------------|--------------------------------|
| `history` | Show version log for a document    | `history [--limit N] <path>`    |
| `diff`    | Compare document versions          | `diff <path>` or `diff a:1 b:2`|
| `graph`   | How the store evolved over time    | `graph [path]`                 |

### Views

| Command  | Description                        | Usage                            |
|----------|------------------------------------|----------------------------------|
| `status` | Store overview dashboard           | `status [--limit N]`            |
| `review` | Pending tasks with context         | `review [--column name] [--limit N]`  |

### Tags and links

| Command  | Description                 | Usage                              |
|----------|-----------------------------|------------------------------------|
| `tag`    | Add, remove, or find tags   | `tag <path> <name>`, `tag -f name` |
| `link`   | Create links between docs   | `link <from> <to>`                 |
| `unlink` | Remove a link               | `unlink <from> <to>`               |

### Tasks, audits, queue

| Command | Description                            | Usage                             |
|---------|----------------------------------------|-----------------------------------|
| `task`  | Manage tasks on the board              | `task <subcommand> [options]`     |
| `audit` | Review threads on tasks and documents  | `audit <subcommand> [options]`    |
| `queue` | Messages between agents and people     | `queue <subcommand> [options]`    |

Task subcommands: add, list, show, next, move, set, rm, restore,
start, finish, branch, diff, files, commits, column, link, links, log,
chain, ready. Audit: add, reply, list, show, resolve, rm, restore,
status. Queue: send, ls, peek, ack, history. See `guide task`,
`guide audit`, `guide queue`.

### Agents and automation

| Command | Description                            | Usage                             |
|---------|----------------------------------------|-----------------------------------|
| `agent` | Run agents on tasks in git worktrees   | `agent <subcommand> [options]`    |
| `rule`  | What happens when a task enters a column | `rule <subcommand> [options]`   |
| `hook`  | Handle an agent platform's session events | `hook [--task id]`, `hook init <platform>` |

See `guide agent`, `guide rule`, `guide hooks` and `guide workflow`.

### Bulk operations

| Command  | Description                        | Usage                        |
|----------|------------------------------------|------------------------------|
| `import` | Import .md files from a directory  | `import [--prefix p] <dir>`  |
| `export` | Export documents to the filesystem | `export <path> [dir]`        |
| `mirror` | Sync documents with .llmd/<db>/    | `mirror [pull\|push] [path]` |

### Admin

| Command   | Description                     | Usage               |
|-----------|---------------------------------|---------------------|
| `init`    | Create a new store              | `init`              |
| `config`  | Read or write configuration     | `config [key] [val]`|
| `vacuum`  | Permanently purge deleted docs  | `vacuum`            |
| `mcp`     | Start MCP server (stdio)        | `mcp`               |
| `serve`   | Start HTTP API server           | `serve`             |
| `version` | Show version information        | `version`           |
| `extensions` | List loaded extensions       | `extensions`        |
| `guide`   | Built-in documentation          | `guide [topic]`     |

## Global flags

- `--author <name>` - who is making the change; required for agents and scripts
- `--json` - structured JSON instead of text
- `--db <path>` - use a different database file
- `--verbose` - debug logging on stderr

## More help

- `llmd guide <topic>` - detailed help on a topic: any command name, plus
  workflow, events, hooks, claude-code and install
- `llmd <command> --help` - usage for a specific command
