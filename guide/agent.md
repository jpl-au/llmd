# llmd agent

Register AI agents and run them on tasks, each in its own git worktree.

## Usage

```
llmd agent add <name>                       Register an agent
llmd agent rm <name>                        Remove an agent
llmd agent ls                               List registered agents
llmd agent config <name>                    Show agent configuration
llmd agent prompt <name> <role>             Show prompt template
llmd agent spawn <task-key> <agent>         Spawn agent for a task
llmd agent run <task-key> --worktree <path> Run agent lifecycle (internal)
llmd agent runs [--status S] [--task K]     List agent runs
llmd agent complete <task-key> --exit-code  Record run completion
llmd agent stop <task-key>                  Stop a running agent
```

Built-in agents: `claude-code`, `gemini`, `aider`. `llmd task start
<id> --agent <name>` spawns the same way as `agent spawn`.

## Flags

| Flag | Subcommands | Description |
|------|-------------|-------------|
| `--command <bin>` | add | Command to run, for agents llmd does not know |
| `--status <s>` | runs | Filter runs by status: running, completed, failed, stopped |
| `--task <key>` | runs | Filter runs by task key |
| `--agent <name>` | runs | Filter runs by agent name |
| `--exit-code <n>` | complete | Process exit code to record |
| `--worktree <path>` | run | The worktree the agent runs in (internal) |

## Rules

- Changing anything (`add`, `rm`, `spawn`, `complete`, `stop`) needs
  an author. `ls`, `config`, `prompt` and `runs` need none.
- `spawn` needs git. It refuses when the task's dependencies are not
  done, and a task can have only one running agent at a time.
- A spawn creates a branch if the task has none, a worktree at
  `.llmd/worktrees/<task-key>/`, a run config `.llmd-run.json` in it,
  and starts `llmd agent run` as a detached process.
- Where the task goes when the agent exits comes from the column's
  rule (`guide rule`): success and failure columns. On failure the
  agent's last output is recorded as an audit note on the task.
- Worktrees live for the whole pipeline (developer, tester, auditor
  share one) and are removed by `task finish`, not when a run ends.
- Agent files are plain text under `.llmd/agents/<name>/`: edit them
  with any editor.
- Piped, `ls`, `config` and `runs` print tab-separated lines with a
  header line. `--json` gives the structured form.

## Examples

```bash
# Register agents
llmd --author alice agent add claude-code
llmd --author alice agent add gemini
llmd --author alice agent add aider

# A tool llmd does not know
llmd --author alice agent add my-agent --command ./my-tool

# Look at what was written
llmd agent config claude-code
llmd agent prompt claude-code developer
vi .llmd/agents/claude-code/developer.md

# Spawn an agent for a task, either way
llmd --author alice agent spawn <task-key> claude-code
llmd --author alice task start <task-key> --agent claude-code

# Set up an automated pipeline via rules
llmd --author alice rule set code --agent claude-code --role developer
llmd --author alice rule set review --agent gemini --role auditor

# Create a task and kick off the pipeline
llmd --author alice task add "Fix auth bug" < spec.md
llmd --author alice task move <key> code

# Monitor progress
llmd agent runs
llmd agent runs --status running
llmd agent runs --task a1b2c3d4e
llmd task board
```

## What a spawn does

`agent spawn <task> <agent>` and `task start <task> --agent <agent>`
do the same thing:

1. Check that the task's dependencies are satisfied
2. Create a git branch if the task doesn't have one
3. Create an isolated git worktree at `.llmd/worktrees/<task-key>/`
4. Assemble the context prompt from the task's spec and templates
5. Write a run config (`.llmd-run.json`) to the worktree
6. Start `llmd agent run` as a detached process
7. Record the run in the agent_runs table
8. Make the agent the assignee and move the task to in-progress.
   Auditors skip this step: the task stays where it is.

The `agent run` process then starts the actual agent (claude, gemini,
etc.), waits for it to finish, records completion stats, captures
failure output as an audit note, and moves the task based on the
exit code and rule transitions.

When a column has a rule with an agent configured, the agent is
spawned automatically when a task enters that column. See
`guide rule`.

## Agent files

Registering an agent creates files on disk:

```
.llmd/agents/<name>/
  config.json         Configuration (command, args)
  settings.json       Runtime settings (permissions, hooks)
  developer.md        Developer prompt template
  developer-http.md   Developer HTTP transport template
  tester.md           Tester prompt template
  auditor.md          Auditor prompt template
  auditor-http.md     Auditor HTTP transport template
```

For the built-in agents llmd knows the binary, arguments and prompt
patterns, and seeds default templates and settings.

## Agent roles

| Role | Purpose |
|------|---------|
| `developer` | Implements the task spec. Works in an isolated worktree. |
| `tester` | Writes and runs tests against the developer's implementation. |
| `auditor` | Reviews changes against the spec. Approves or rejects. |

The role determines which prompt template is used and how task
transitions are handled after the agent exits.

## Prompt templates

Templates use Go `text/template` syntax. Available fields:

| Field | Description |
|-------|-------------|
| `{{.Key}}` | Task ID |
| `{{.Title}}` | Task title |
| `{{.Branch}}` | Git branch name |
| `{{.AssignedTo}}` | Who the task is assigned to |
| `{{.Agent}}` | Agent name |
| `{{.URL}}` | HTTP API URL (if server is running) |
| `{{.SpecPath}}` | Path to the task's spec document |
| `{{.OnSuccess}}` | Column to move to on success (from rule) |
| `{{.OnFailure}}` | Column to move to on failure (from rule) |

When spawning, the template is resolved in order:

1. `.llmd/agents/<name>/<role>.md` - agent-specific template
2. `.llmd/agents/default/<role>.md` - shared default template
3. Built-in embedded template

If the HTTP server is running, the `-http` variant is tried first
(e.g. `developer-http.md` before `developer.md`).

## Run tracking

Every agent spawn is recorded across two insert-only tables:

**`agent_runs`** holds the immutable identity created at spawn time:
- Agent name, git branch, and worktree path
- Author and start timestamp

**`agent_events`** holds lifecycle transitions (spawned, completed,
failed, stopped). Terminal events carry:
- Exit code and timestamps
- Monetary cost, input/output token counts, model name
- Session ID (for resuming the agent's conversation context)

| Status | Meaning |
|--------|---------|
| `running` | Agent process is active |
| `completed` | Agent exited successfully (exit code 0) |
| `failed` | Agent exited with an error |
| `stopped` | Agent was manually terminated |

When an agent process exits, `llmd agent run` records the result
automatically. It extracts stats (cost, tokens, model) from the
agent's output log and updates the run record. On failure, the last
output from the agent log is captured as an audit note on the task
so humans can see what went wrong. The task is then moved to the
next column based on the exit code and the rule's success/failure
transitions.

## Platform support

llmd detects the agent platform and handles differences
automatically:

| Platform | Budget flags | Cost extraction | Session resume | Settings location |
|----------|-------------|-----------------|----------------|-------------------|
| Claude Code | `--max-budget-usd` | JSON output (`total_cost_usd`) | `--resume <id>` | `.claude/settings.json` |
| Gemini CLI | - | Token counts from JSON | - | - |
| Generic | - | - | - | - |

## Session resumption

When an agent completes a run, its session ID is captured from the
tool's JSON output and stored in the completion event. Later spawns
can resume that conversation instead of starting fresh.

To enable resume for a task, set the `resume` flag:

```bash
llmd --author alice task set <task-key> --flag resume
```

When the task is next spawned (manually or via pipeline), llmd checks
for a previous session ID. If one exists and the platform supports
resume, the agent is started with resume arguments (e.g.
`claude --resume <session-id>`) instead of a fresh prompt. This is
useful when an audit finds issues and the original agent needs to
fix its own work with full context of what it already did.

To disable resume and force a fresh spawn:

```bash
llmd --author alice task set <task-key> --unflag resume
```

See `guide rule` for column automation and `guide task` for task
management.
