# Claude Code integration

How llmd works inside Claude Code: as an MCP server, with hooks, and
alongside Claude Code's own `/schedule` and `/loop` commands.

## Quick reference

```
"llmd": {"command": "llmd", "args": ["mcp"]}   MCP server entry for Claude Code settings (tools appear as mcp__llmd__<tool>)
llmd_init                              Initialise a store from inside a session
llmd hook init claude                  Print the hooks section for .claude/settings.json
/schedule "..."                        Recurring work on a cron, outside any session
/loop 2m ...                           Repeat a prompt in the current session
```

| Mechanism | Fires on | Use for |
|-----------|----------|---------|
| Column rules | Task enters a column | Automated pipeline stages (build, test, review) |
| `/schedule` | Cron schedule | Recurring maintenance (audits, stale checks, reports) |
| `/loop` | Interval in active session | Real-time monitoring (agent runs, board changes) |
| Queue messages | Manual or automated send | Cross-agent coordination and notifications |

Rules:

- Every MCP tool call that changes something carries `author`; the
  server instructions and `guide` say how. See `guide mcp`.
- MCP tools return text. For structured output a scheduled agent can
  parse, run the command in a shell with `--json`.
- Hooks run `llmd --author claude hook`; only session start prints
  anything, a count and the newest pending queue messages, so the
  agent's context stays small. See `guide hooks`.

## How it works

llmd registers as an MCP server via Claude Code's settings. Once
configured, llmd's tools are available with the `mcp__llmd__` prefix:
most commands, with grep, find, glob and init as `llmd_grep` and so on
(`guide mcp` lists which commands are left out). A companion skill definition (`skills/SKILL.md`)
teaches Claude Code when and how to use llmd tools - it activates
automatically when a request involves documents, tasks, audits,
agents, or search.

To initialise a store in the current directory:

```
llmd_init
```

## Composing with /schedule

Claude Code's `/schedule` command creates remote agents that run on a
cron schedule without an active session. Scheduled triggers can call
llmd tools, enabling recurring workflows that complement the
task-movement automation provided by column rules.

Column rules fire when a task moves between columns. Scheduled triggers
handle the work that falls outside that model: periodic health checks,
stale task detection, and recurring reports.

### Periodic audit sweeps

Schedule a weekly check for unresolved audit threads:

```
/schedule "Every Monday at 9am, check for pending audits with
audit list --pending. For any unresolved audit older than 3 days,
send a reminder via queue send --assign <author> with the audit ID
and a note that it is awaiting response."
```

### Stale task detection

Schedule a daily scan for tasks that have not moved:

```
/schedule "Every weekday at 8am, list tasks with llmd task list --json.
Flag any task that has been in in-progress or review for more than
48 hours by adding an audit note: audit add <task-id> with a
message noting the task appears stale."
```

### Board health reports

Schedule a weekly summary written back into the store:

```
/schedule "Every Friday at 5pm, review the task board and pending
audits. Write a summary to reports/weekly/<date> covering: tasks
completed this week, tasks still in progress, and any unresolved
audit threads."
```

## Composing with /loop

Claude Code's `/loop` command runs a prompt on a recurring interval
within the current session. This is useful for monitoring active work
in real time.

### Monitoring agent runs

While agents are working on tasks, poll their status:

```
/loop 2m Check agent run status with agent runs --status running.
Report any changes since the last check.
```

### Watching the board

During a sprint or review cycle, keep a live view of board movement:

```
/loop 5m Show the current task board with task list. Highlight any
tasks that moved since the last check.
```

### Tracking audit responses

While waiting for review feedback on a specific task:

```
/loop 3m Check for new audit activity with audit status. Alert me
when there are new replies or resolutions.
```

## When to use what

Column rules, `/schedule`, `/loop` and queue messages compose
naturally. A scheduled trigger might detect a stale task and send a
queue message. A column rule might spawn an agent that writes an
audit. A `/loop` monitor might surface a failed run for human
intervention.

## Multi-agent workflow example

Combining all three mechanisms for a fully automated pipeline:

```bash
# 1. Set up the pipeline with column rules
llmd rule set code --agent claude-code --role developer
llmd rule set test --agent claude-code --role tester
llmd rule set review --agent gemini --role auditor
```

```
# 2. Schedule recurring maintenance
/schedule "Every weekday at 8am, check for stale tasks and
unresolved audits. Send queue messages for anything needing
attention."
```

```
# 3. Monitor in real time when actively working
/loop 2m Show agent runs and any tasks that moved since last check.
```

```bash
# 4. Kick off work - automation handles the rest
llmd task add "Implement auth refresh" < spec.md
llmd task move <key> code
```

The pipeline runs autonomously through column rules. Scheduled triggers
handle the operational housekeeping. In-session loops provide visibility
when you need it.

## Hooks

Claude Code can run a command at session start and end, on task
completion and after tool use. Generate the configuration and merge it
into `.claude/settings.json`:

```bash
llmd hook init claude
```

Each hook runs `llmd --author claude hook`, which reads Claude Code's
payload and acts on it: session start prints a count and the newest
pending queue messages into the context (`hook.limit`, default 10).
Task events need to know the task: in the worktree of an agent llmd
spawned, the hooks carry `--task`, so finishing the session finishes
that task. Your own session's hooks from `hook init` name no task, so
only session start does anything there.
No server is needed. See `guide hooks` for every event.

## Tips

- Use `--json` on the command line for structured output that
  scheduled agents can parse and act on
- Queue messages (`queue send --assign`) are a good way for scheduled
  agents to notify humans or other agents about issues
- Audit threads provide a persistent record of agent decisions and
  review feedback, making them ideal for scheduled review sweeps
- Use `guide <topic>` to look up command details when building
  scheduled prompts

See `guide agent` for agent registration, `guide rule` for column
automation, and `guide workflow` for end-to-end workflows.
