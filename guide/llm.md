# LLM integration guide

How to use llmd from an AI agent.

## MCP server (recommended)

The fastest way to integrate is via MCP. Add llmd as an MCP server and
the agent gets direct access to document tools.

Start the server:

```
llmd mcp
```

Or equivalently:

```
llmd serve
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

Each tool accepts `args` (array of strings) and `content` (string, used
by write and edit for document bodies).

## CLI fallback

When MCP is not available, shell out to the llmd binary:

```
llmd cat notes/readme
llmd ls -l projects/
echo "new content" | llmd write notes/readme
llmd grep "TODO" projects/
llmd edit notes/readme "old text" "new text"
```

Use `--json` for structured output that is easier to parse:

```
llmd ls --json
llmd history --json docs/spec
llmd grep --json "budget"
```

## Configuration

Author must be set before writing. Configure it once:

```
llmd config author "Claude"
```

This writes to `.llmd/config`. The author name is recorded on every
write, edit, tag, and link operation.

## Common patterns

### Read a document

```
llmd cat notes/readme
llmd cat --version 2 notes/readme       # specific version
llmd cat -n notes/readme                # with line numbers
```

### Write or update

```
echo "content here" | llmd write path/to/doc
echo "updated" | llmd write path/to/doc --message "why it changed"
```

### Search

```
llmd grep "error handling"              # full-text search
llmd find "authentication" projects/    # paths only
llmd glob "specs/*.md"                  # path pattern match
```

### Edit in place

```
llmd edit notes/readme "old text" "new text"
llmd sed 's/oldterm/newterm/' notes/readme
```

### Organise with tags

```
llmd tag notes/readme important
llmd tag -f important                   # find tagged documents
llmd tag -d notes/readme important      # remove tag
```

### Review history

```
llmd history notes/readme
llmd diff notes/readme                  # what changed last
llmd diff notes/readme:1 notes/readme:3 # compare versions
```

## More help

- `llmd guide` — full command reference
- `llmd guide workflow` — best practices
- `llmd <command> --help` — usage for a specific command
