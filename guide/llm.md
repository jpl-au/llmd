# LLM integration guide

How to use llmd from an AI agent.

## MCP server (recommended)

The fastest way to integrate is via MCP. Add llmd as an MCP server and
the agent gets direct access to document tools.

Start the server:

```
llmd mcp
```

This exposes tools over stdio using the Model Context Protocol. Tool
names match CLI commands, except where they would collide with common
tool names:

| CLI command | MCP tool name |
|-------------|---------------|
| `cat`       | `cat`         |
| `write`     | `write`       |
| `edit`      | `edit`        |
| `ls`        | `ls`          |
| `grep`      | `llmd_grep`   |
| `find`      | `llmd_find`   |
| `glob`      | `llmd_glob`   |
| `tag`       | `tag`         |
| `link`      | `link`        |
| `history`   | `history`     |
| `diff`      | `diff`        |
| `rm`        | `rm`          |
| `restore`   | `restore`     |
| `revert`    | `revert`      |
| `mv`        | `mv`          |
| `sed`       | `sed`         |
| `unlink`    | `unlink`      |
| `audit`     | `audit`       |
| `task`      | `task`        |

Each tool accepts `args` (array of strings) and `content` (string, used
by write and edit for document bodies).

## HTTP API

When the agent can make HTTP requests (e.g. via a tool or fetch), the
HTTP server provides a REST interface to all commands.

Start the server:

```
llmd serve
```

Routes follow a simple pattern: read commands are `GET /<command>/<path>`,
mutation commands are `POST /<command>/<path>`. Headers carry metadata.

| Action                | Request                                               |
|-----------------------|-------------------------------------------------------|
| Read a document       | `GET /cat/docs/readme`                                |
| Read a version        | `GET /cat/docs/readme?version=2`                      |
| List documents        | `GET /ls`                                             |
| Search                | `GET /grep?q=authentication`                          |
| Write a document      | `POST /write/docs/readme` with body and `Author` header |
| Delete a document     | `POST /rm/docs/readme` with `Author` header           |
| Version history       | `GET /history/docs/readme`                            |

Headers:

| Header   | Purpose                                        |
|----------|------------------------------------------------|
| `Author` | Required for mutations. Identifies the agent   |
| `Message`| Version message for writes                     |
| `Output` | Set to `json` to force JSON responses          |

Query parameters map to command flags: `?version=3` becomes `--version 3`,
`?n=5` becomes `-n 5`. The `q` parameter is special — it becomes the
search pattern for grep and find.

See `llmd guide serve` for the full route reference.

## CLI fallback

When MCP is not available, shell out to the llmd binary. Read commands
work without `--author`; write commands require it:

```
llmd cat notes/readme
llmd ls -l projects/
echo "new content" | llmd --author "Claude" write notes/readme
llmd grep "TODO" projects/
llmd --author "Claude" edit notes/readme "old text" "new text"
```

Use `--json` for structured output that is easier to parse:

```
llmd ls --json
llmd history --json docs/spec
llmd grep --json "budget"
```

## Author attribution

LLMs and scripts must pass `--author` on mutation commands. Do not
pass `--author` on read commands — it is not needed.

**Do not use `llmd config author` to set yourself as the author.** The
config author is reserved for the human user. LLMs must always pass
`--author` per command so that mutations are attributed correctly and
the human's identity is not overwritten.

The `--author` flag goes before the command name:

```
llmd --author "Claude" write notes/readme
```

### Command reference: author requirements

| Command              | Needs `--author` | Notes                          |
|----------------------|-------------------|--------------------------------|
| `cat`                | No                | Read document                  |
| `ls`                 | No                | List documents                 |
| `grep`               | No                | Full-text search               |
| `find`               | No                | Search, paths only             |
| `glob`               | No                | Path pattern match             |
| `history`            | No                | Version history                |
| `diff`               | No                | Compare versions               |
| `status`             | No                | Dashboard overview             |
| `review`             | No                | Pending tasks                  |
| `task list`          | No                | List tasks                     |
| `task show`          | No                | Show a task                    |
| `task column list`   | No                | List columns                   |
| `audit list`         | No                | List audits                    |
| `audit show`         | No                | Show audit thread              |
| `tag -f <name>`      | No                | Find documents by tag          |
| `tag <path>`         | No                | List tags on a document        |
| `link <path>`        | No                | List links on a document       |
| `write`              | Yes           | Create or update a document    |
| `edit`               | Yes           | Search and replace             |
| `sed`                | Yes           | Sed-style substitution         |
| `rm`                 | Yes           | Soft-delete a document         |
| `mv`                 | Yes           | Move or rename                 |
| `restore`            | Yes           | Recover a deleted document     |
| `revert`             | Yes           | Roll back to a previous version|
| `import`             | Yes           | Import .md files               |
| `tag <path> <name>`  | Yes           | Add a tag                      |
| `tag -d <path>`      | Yes           | Remove a tag                   |
| `link <from> <to>`   | Yes           | Create a link                  |
| `unlink`             | Yes           | Remove a link                  |
| `task add`           | Yes           | Create a task                  |
| `task move`          | Yes           | Move a task between columns    |
| `task set`           | Yes           | Update task fields             |
| `task rm`            | Yes           | Delete a task                  |
| `task start`         | Yes           | Start work on a task           |
| `task finish`        | Yes           | Complete a task                |
| `task branch`        | Yes           | Create a branch for a task     |
| `audit add`          | Yes           | Create an audit                |
| `audit reply`        | Yes           | Reply to an audit thread       |
| `audit resolve`      | Yes           | Mark audit as approved         |
| `audit rm`           | Yes           | Soft-delete an audit           |
| `audit status`       | Yes           | Inbox (filtered by author)     |

## Common patterns

### Read a document

```
llmd cat notes/readme
llmd cat --version 2 notes/readme       # specific version
llmd cat -n notes/readme                # with line numbers
```

### Write or update

```
echo "content here" | llmd --author "Claude" write path/to/doc
echo "updated" | llmd --author "Claude" write path/to/doc --message "why it changed"
```

### Search

```
llmd grep "error handling"              # full-text search
llmd find "authentication" projects/    # paths only
llmd glob "specs/*.md"                  # path pattern match
```

### Edit in place

```
llmd --author "Claude" edit notes/readme "old text" "new text"
llmd --author "Claude" sed 's/oldterm/newterm/' notes/readme
```

### Organise with tags

```
llmd --author "Claude" tag notes/readme important    # add (mutation)
llmd tag -f important                                # find (read-only)
llmd --author "Claude" tag -d notes/readme important # remove (mutation)
```

### Review history

```
llmd history notes/readme
llmd diff notes/readme                  # what changed last
llmd diff notes/readme:1 notes/readme:3 # compare versions
```

### Audits

```
llmd --author "Claude" audit add docs/spec "Needs error handling."
llmd --author "Claude" audit reply 0mmsfn7h1 "Fixed."
llmd --author "Claude" audit resolve 0mmsfn7h1
llmd audit list docs/spec
llmd audit show 0mmsfn7h1
llmd --author "Claude" audit status
```

### Polling for changes

Use `--since` to cheaply check for new activity without fetching
everything. Accepts a duration (`5m`, `1h`) or RFC 3339 timestamp:

```
llmd ls --since 5m                     # new/updated documents
llmd task list --since 5m              # recently created tasks
llmd audit list --since 5m             # recent audits
llmd --author "Claude" audit status --since 5m  # recent inbox items
```

## More help

- `llmd guide` — full command reference
- `llmd guide workflow` — best practices
- `llmd <command> --help` — usage for a specific command
