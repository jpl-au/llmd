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

## What the agent sees

An MCP client reads two things from llmd when a session starts, and
both cost context in every session, so both are kept short:

- **Instructions** - a few lines on the calling convention, document
  paths, the task loop (`task next`, work, `task move <id> review`)
  and where to find help. The client shows them to the model once.
- **The tool list** - one tool per command with a short description:
  a summary, the usage line, the flags and `Details: guide <command>`.
  The prose that `llmd <command> --help` shows is left out; the model
  calls `guide` when it needs it. The whole list is held under 12 KB
  by a test.

## Notes

- Communicates over stdin/stdout using the MCP protocol.
- Exposes llmd commands as MCP tools.
- Tool names match command names, except: `grep` becomes `llmd_grep`,
  `find` becomes `llmd_find`, `glob` becomes `llmd_glob`, `init` becomes
  `llmd_init`.
- `args`, `content` and `author` are all optional. Read-only tools need
  no author; mutations without one fall back to the author the server
  was started with.

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
