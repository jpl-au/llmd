# Agent hooks

One command, `llmd hook`, that agent platforms such as Claude Code and
Gemini CLI run at points in a session, so llmd can act on what happened.

## Quick reference

```
llmd --author <name> hook [--task <id>]    Handle a hook payload read from stdin
llmd hook init <platform>                  Print hook configuration: claude, gemini, generic
```

| Event | What llmd does |
|-------|----------------|
| Session start | Prints how many queue messages await the author and the newest few, so the agent sees what needs attention as it begins |
| Session end or stop | Sends a queue message saying the session ended |
| Task completed | Finishes the task and sends a queue message |
| Task failed | Moves the task to `blocked` and sends a queue message |
| Tool use | Records an audit entry on the task, when the payload names a task and has content |

## Usage

```
llmd --author <name> hook
llmd --author <name> hook --task <id>
llmd hook init claude
llmd hook init gemini
llmd hook init generic
```

## Flags

| Flag | Description |
|------|-------------|
| `--task <id>` | Task the event belongs to, when the payload does not say |

## Rules

- `--author` is required. It names the agent and selects the payload
  format: a name containing `claude` or `gemini` uses that platform's
  format, anything else uses the generic format, a JSON object with
  `event`, `task_id` and `content` fields.
- The payload comes on stdin; an empty stdin is an error.
- Events not in the table are accepted and ignored. A step that fails,
  such as finishing a task that is not ready, is logged and skipped so
  the rest still happens.
- Only session start prints anything. Keep it that way: platforms
  inject hook output into the agent's context, so quiet hooks keep the
  context small.
- Session start prints a count and the newest 10 pending messages, one
  per line as key and payload; `queue ls` lists the rest. The number is
  the `hook.limit` config key; `0` prints every message. Nothing is
  printed when the queue is empty.
- llmd finds its store under `.llmd/` in the current directory. If an
  agent runs elsewhere, prefix the command with `cd` to the project.
- The same handler sits behind every way of reaching llmd: the command
  line, `POST /hook` over HTTP, and the settings written for a spawned
  agent all do the same thing.

## Examples

```bash
llmd hook init claude     # a hooks section for .claude/settings.json
llmd hook init gemini     # a hooks list for the Gemini CLI configuration
llmd hook init generic    # a shell script that wraps any command

# Run any agent through the generic script, task key first
./hooks.sh 0mvj8snvv7 my-agent --some-flag

# A generic payload any tool can produce
printf '{"event":"task.completed","task_id":"0mvj8snvv7"}' | llmd --author my-agent hook

# Name the task when the platform's payload does not
llmd --author claude hook --task 0mvj8snvv7
```

## Setting up a platform

Print the configuration for your platform and install it where the
platform expects. The author in the generated configuration is the
configured author if one is set (`llmd config author`), otherwise the
platform name.

### Claude Code

Merge the output of `llmd hook init claude` into `.claude/settings.json`.
It runs the hook command at session start and end, on task completion,
and after calls to llmd's MCP tools.

### Gemini CLI

Add the output of `llmd hook init gemini` to your hooks configuration.
It runs the hook command at session start and after each tool use.

### Any other agent

`llmd hook init generic` prints a shell script. Run your agent through
it with the task key as the first argument. The script reports a
session start before the command runs, then a completed or failed task
depending on the exit code. Its payloads use the generic format, which
any tool can produce.

## Naming the task

Some platforms do not say which task an event belongs to. Pass it with
`--task`. Agents that llmd spawns receive this automatically in the
settings written into their working directory.

## Over HTTP

When `llmd serve` is running, `POST /hook` is the same command: the
body is the payload and the `Author` header names the agent. See
`guide serve`.
