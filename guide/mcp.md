# llmd mcp

Start an MCP (Model Context Protocol) server over stdio.

## Usage

```
llmd mcp
```

## Examples

```bash
# Start the MCP server
llmd mcp
```

## Notes

- Communicates over stdin/stdout using the MCP protocol.
- Exposes llmd commands as MCP tools.
- Tool names match command names, except: `grep` becomes `llmd_grep`,
  `find` becomes `llmd_find`, `glob` becomes `llmd_glob`.

## Tool input schema

All tools accept a JSON input with the following fields:

| Field | Description |
|-------|-------------|
| `args` | Command arguments (array of strings) |
| `content` | Document content for write/edit (string) |
| `author` | **Required for mutations.** Identifies the LLM or agent making the change. |

Read-only tools (cat, ls, grep, etc.) do not require `author`. All mutation
tools (write, edit, rm, mv, tag, link, task, audit, etc.) will reject calls that
do not include `author`.

```json
{"author": "Claude", "args": ["notes/summary"], "content": "Meeting notes..."}
```

The `author` field exists so that changes made by LLMs are correctly
attributed and distinguishable from human-authored changes in the version
history.
