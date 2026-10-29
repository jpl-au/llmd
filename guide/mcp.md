# llmd mcp

Run llmd as an MCP server over stdin and stdout, so an agent's client
calls every llmd command as a tool.

## Usage

```
llmd mcp
```

## Rules

- Every tool takes the same three fields: `args`, the command-line
  arguments as a list of strings; `content`, text that would have come
  on stdin, such as a document body; and `author`, who is acting.
- `author` is required on every call that changes something (write,
  edit, rm, mv, tag, link, task, audit and so on). Reads need none.
  A mutation with no author falls back to the author the server was
  started with, and is rejected if there is none.
- Tool names match command names, except four that would clash with an
  agent's own tools: `llmd_grep`, `llmd_find`, `llmd_glob`, `llmd_init`.
- Not every command is a tool. Those for a person at a terminal or for
  setup (`status`, `review`, `import`, `export`, `vacuum`, `mirror`,
  `hook`, `config`, `serve`, `version`, `extensions`) are left out to
  keep the tool list, which every session reads, small.
- Responses are the plain text a piped `llmd` prints: raw markdown from
  `cat`, `guide` and `task show`, tab-separated lines from tables, one
  seven-field line per task from `task list`. There is no `--json` over
  MCP; a capped list ends with a note of how many were left out.
- The server talks only over stdin and stdout; the agent's client
  starts it, not you.

## Examples

```bash
# Start the MCP server (an MCP client does this for you)
llmd mcp
```

```json
{"author": "Claude", "args": ["notes/summary"], "content": "Meeting notes..."}
```

## What the agent sees

An MCP client reads two things from llmd when a session starts, and
both cost context in every session, so both are kept short:

- **Instructions** - a few lines on the calling convention, document
  paths, the task loop (`task next`, work, `task move <id> review`)
  and where to find help. The client shows them to the model once.
- **The tool list** - one tool per command with a short description:
  a summary, the usage line, the flags and `Details: guide <command>`.
  The prose that `llmd <command> --help` shows a person at a terminal
  is left out; the model calls `guide` when it needs it. The whole
  list is held under 14 KB by a test.

The tool description is the same text a model gets from `llmd
<command> --help` when llmd is run in a shell and its output is piped:
one agent help, two ways to read it.

## Tool input schema

| Field | Description |
|-------|-------------|
| `args` | Command arguments (array of strings) |
| `content` | Document content for write/edit (string) |
| `author` | **Required for mutations.** Identifies the LLM or agent making the change. |

The `author` field exists so that changes made by LLMs are correctly
attributed and distinguishable from human-authored changes in the version
history.
