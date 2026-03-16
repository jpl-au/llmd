# llmd guide

llmd is a versioned document store. Documents are plain text, addressed by
path, with automatic versioning and full-text search.

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
| `history` | Show version log for a document    | `history [-n limit] <path>`    |
| `diff`    | Compare document versions          | `diff <path>` or `diff a:1 b:2`|

### Views

| Command  | Description                        | Usage                            |
|----------|------------------------------------|----------------------------------|
| `status` | Store overview dashboard           | `status [-n limit]`              |
| `review` | Review pending tasks with context  | `review [--column name] [-n N]`  |

### Tags and links

| Command  | Description                 | Usage                              |
|----------|-----------------------------|------------------------------------|
| `tag`    | Add, remove, or find tags   | `tag <path> <name>`, `tag -f name` |
| `link`   | Create links between docs   | `link <from> <to>`                 |
| `unlink` | Remove a link               | `unlink <from> <to>`               |

### Tasks

| Command | Description                            | Usage                             |
|---------|----------------------------------------|-----------------------------------|
| `task`  | Manage tasks on the board              | `task <subcommand> [options]`     |

Subcommands: add, list, show, move, set, rm, restore, start, diff,
files, column, link, links, log. See `llmd guide task`.

### Audits

| Command | Description                            | Usage                             |
|---------|----------------------------------------|-----------------------------------|
| `audit` | Agent-to-agent review threads          | `audit <subcommand> [options]`    |

Subcommands: add, reply, list, show, resolve, rm, restore, status.
See `llmd guide audit`.

### Bulk operations

| Command  | Description                        | Usage                        |
|----------|------------------------------------|------------------------------|
| `import` | Import .md files from a directory  | `import [--prefix p] <dir>`  |
| `export` | Export documents to the filesystem | `export <path> [dir]`        |
| `mirror` | One-way snapshot to .llmd/mirror/  | `mirror [path]`              |

### Admin

| Command   | Description                     | Usage               |
|-----------|---------------------------------|---------------------|
| `init`    | Create a new store              | `init`              |
| `config`  | Read or write configuration     | `config [key] [val]`|
| `vacuum`  | Permanently purge deleted docs  | `vacuum`            |
| `mcp`     | Start MCP server (stdio)        | `mcp`               |
| `serve`   | Start HTTP API server           | `serve`             |
| `version` | Show version information        | `version`           |
| `plugins` | List loaded plugins             | `plugins`           |
| `guide`   | Built-in documentation          | `guide [topic]`     |
| `llm`     | Quick reference for LLMs        | `llm`               |

## Global flags

- `--author <name>` — author for mutations (required for LLMs and scripts)
- `--json` — output structured JSON instead of text
- `--db <path>` — use a different database file

## More help

- `llmd guide <topic>` — detailed help on a topic (workflow, llm, install)
- `llmd <command> --help` — usage for a specific command
