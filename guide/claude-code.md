# Claude Code integration

llmd runs as an MCP server inside Claude Code sessions, exposing its
document store, task board, and agent orchestration as MCP tools. This
guide covers how to compose llmd with Claude Code's built-in scheduling
and monitoring to create autonomous workflows.

## How it works

llmd registers as an MCP server via Claude Code's settings. Once
configured, all llmd commands are available as MCP tools with the
`mcp__llmd__` prefix. A companion skill definition (`skills/SKILL.md`)
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
/schedule "Every weekday at 8am, list tasks with task list --json.
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

| Mechanism | Fires on | Use for |
|-----------|----------|---------|
| Column rules | Task enters a column | Automated pipeline stages (build, test, review) |
| `/schedule` | Cron schedule | Recurring maintenance (audits, stale checks, reports) |
| `/loop` | Interval in active session | Real-time monitoring (agent runs, board changes) |
| Queue messages | Manual or automated send | Cross-agent coordination and notifications |

These mechanisms compose naturally. A scheduled trigger might detect a
stale task and send a queue message. A column rule might spawn an agent
that writes an audit. A `/loop` monitor might surface a failed run for
human intervention.

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

Claude Code supports HTTP hooks that POST lifecycle events to an
endpoint. When the llmd HTTP server is running, hooks can POST
directly to the `/hook` endpoint for real-time integration.

Generate the hook configuration:

```bash
llmd hook init claude
```

This outputs a settings.json snippet with HTTP hooks for session
start, session end, task completion, and tool use. Merge it into
your `.claude/settings.json`.

The hook endpoint parses Claude Code's native payload format and
routes events to the appropriate SDK operations: session start
returns pending queue messages, task completion finishes the task
and sends a notification, and tool use events are logged as audits.

See `guide serve` for the full `/hook` endpoint reference.

## Tips

- Use `--json` on any tool for structured output that scheduled agents
  can parse and act on
- Queue messages (`queue send --assign`) are a good way for scheduled
  agents to notify humans or other agents about issues
- Audit threads provide a persistent record of agent decisions and
  review feedback, making them ideal for scheduled review sweeps
- Use `guide <topic>` to look up command details when building
  scheduled prompts

See `guide agent` for agent registration, `guide rule` for column
automation, and `guide workflow` for end-to-end workflows.
