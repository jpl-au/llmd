# Agent hooks

Agent platforms such as Claude Code and Gemini CLI can run a command
at points in a session: when it starts, when it ends, after a tool is
used, when a task completes. llmd gives them one command for all of
these:

```
llmd --author <name> hook
```

The platform passes its own event payload on standard input. llmd
works out the platform from the author name, reads the event, and acts
on it. The same handler sits behind every way of reaching llmd, so a
hook over the command line, over HTTP and from a spawned agent all do
the same thing.

## What each event does

| Event | What llmd does |
|-------|----------------|
| Session start | Prints the author's pending queue messages, so the agent sees what needs attention as it begins |
| Session end or stop | Sends a queue message saying the session ended |
| Task completed | Finishes the task and sends a queue message |
| Task failed | Moves the task to `blocked` and sends a queue message |
| Tool use | Records an audit entry on the task, when the payload names a task and has content |

Events the platform sends that are not in this table are accepted and
ignored. A step that fails, such as finishing a task that is not ready,
is logged and skipped so the rest still happens.

## Setting up a platform

Print the configuration for your platform and install it where the
platform expects:

```bash
llmd hook init claude     # a hooks section for .claude/settings.json
llmd hook init gemini     # a hooks list for the Gemini CLI configuration
llmd hook init generic    # a shell script that wraps any command
```

The author in the generated configuration is the configured author if
one is set (`llmd config author`), otherwise the platform name.

### Claude Code

Merge the output of `llmd hook init claude` into `.claude/settings.json`.
It runs the hook command at session start and end, on task completion,
and after calls to llmd's MCP tools.

### Gemini CLI

Add the output of `llmd hook init gemini` to your hooks configuration.
It runs the hook command at session start and after each tool use.

### Any other agent

`llmd hook init generic` prints a shell script. Run your agent through
it with the task key as the first argument:

```bash
./hooks.sh 0mvj8snvv my-agent --some-flag
```

The script reports a session start before the command runs, then a
completed or failed task depending on the exit code. Its payloads use
the generic format, a JSON object with `event`, `task_id` and `content`
fields, which any tool can produce:

```bash
printf '{"event":"task.completed","task_id":"0mvj8snvv"}' | llmd --author my-agent hook
```

## Naming the task

Some platforms do not say which task an event belongs to. Pass it with
`--task`:

```bash
llmd --author claude hook --task 0mvj8snvv
```

Agents that llmd spawns receive this automatically in the settings
written into their working directory.

## Over HTTP

When `llmd serve` is running, `POST /hook` is the same command: the
body is the payload and the `Author` header names the agent. See
`llmd guide serve`.

## Notes

- `--author` is required. It names the agent and selects the payload
  format: a name containing `claude` or `gemini` uses that platform's
  format, anything else uses the generic format.
- Only session start prints anything. Keep it that way: platforms
  inject hook output into the agent's context, so quiet hooks keep the
  context small.
- llmd finds its store under `.llmd/` in the current directory. If an
  agent runs elsewhere, prefix the command with `cd` to the project.
