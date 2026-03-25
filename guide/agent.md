# llmd agent

Manage AI agent configurations and runs. Agents are external tools
(Claude Code, Gemini CLI, Aider, etc.) that can be spawned to work
on tasks in isolated git worktrees.

Agent configurations and prompt templates are stored as documents
in the llmd store, making them versioned, portable, and editable.

## Usage

```
llmd agent add <name>                       Register an agent
llmd agent rm <name>                        Remove an agent
llmd agent ls                               List registered agents
llmd agent config <name>                    Show agent configuration
llmd agent prompt <name> <role>             Show prompt template
llmd agent runs [--status S] [--task K]     List agent runs
llmd agent stop <task-key>                  Stop a running agent
```

## Registering agents

llmd ships with built-in profiles for common agents. Registration
is a single command:

```bash
llmd agent add claude-code
llmd agent add gemini
llmd agent add aider
```

That's it. llmd knows the binary, arguments, and prompt patterns
for each. Default prompt templates are seeded automatically.

### Custom agents

For agents llmd doesn't know about, provide the command:

```bash
llmd agent add my-agent --command ./my-tool
```

### What happens on add

When you register an agent, llmd creates three documents:

```
agents/<name>/config       Configuration (command, args)
agents/<name>/developer    Developer prompt template
agents/<name>/auditor      Auditor prompt template
```

View what was created:

```bash
llmd agent config claude-code
llmd agent prompt claude-code developer
```

Customise a template:

```bash
llmd edit agents/claude-code/developer "old text" "new text"
```

Or rewrite it entirely:

```bash
llmd write agents/claude-code/developer < my-template.md
```

## Spawning agents

Agents are spawned via `task start --assign`:

```bash
llmd task start a1b2c3d4e --assign claude-code
```

This:

1. Checks that the task's dependencies are satisfied
2. Creates a git branch if the task doesn't have one
3. Creates an isolated git worktree at `.llmd/worktrees/<task-key>/`
4. Assembles the context prompt from the task's spec, linked
   documents, and audit history
5. Starts the agent process in the worktree
6. Moves the task to in-progress

The agent works independently. When done, it calls llmd to move the
task (e.g. `llmd task move <id> review`). The worktree is cleaned up
automatically after the agent process exits.

### Budget control

Set a per-spawn budget limit:

```bash
llmd task start a1b2c3d4e --assign claude-code --budget 3.00
```

Budget flags are platform-specific. Currently `--max-budget-usd` is
appended for Claude Code agents.

## Prompt templates

Templates use Go `text/template` syntax. Available fields:

| Field | Description |
|-------|-------------|
| `{{.Key}}` | Task ID |
| `{{.Title}}` | Task title |
| `{{.Branch}}` | Git branch name |
| `{{.AssignedTo}}` | Who the task is assigned to |
| `{{.Agent}}` | Agent name |
| `{{.Spec}}` | Spec document content |
| `{{.Diff}}` | Git diff (for auditor templates) |
| `{{.LinkedDocs}}` | Linked documents (range with `.Path`, `.Content`) |
| `{{.Audits}}` | Audit history (range with `.Author`, `.Status`, `.Content`) |

### Fallback chain

When spawning, the prompt template is resolved in order:

1. `agents/<name>/<role>` - agent-specific template
2. `agents/default/<role>` - shared default template
3. Built-in fallback - hardcoded in llmd

This means you can customise per agent or set project-wide defaults.

## Monitoring runs

```bash
# List all runs
llmd agent runs

# Filter by status
llmd agent runs --status running

# Filter by task
llmd agent runs --task a1b2c3d4e

# Filter by agent
llmd agent runs --agent claude-code
```

### Run statuses

| Status | Meaning |
|--------|---------|
| `running` | Agent process is active |
| `completed` | Agent exited successfully (exit code 0) |
| `failed` | Agent exited with an error |
| `stopped` | Agent was manually terminated |

## Stopping agents

```bash
llmd agent stop a1b2c3d4e
```

This sends SIGTERM to the agent process, marks the run as stopped,
and cleans up the worktree.

## Environment variables

Agents receive these environment variables:

| Variable | Description |
|----------|-------------|
| `LLMD_TASK_ID` | The task key being worked on |
| `LLMD_AGENT` | The agent configuration name |

## Examples

```bash
# Register Claude Code
llmd agent add claude-code

# Create a task and assign it
llmd task add "Implement auth middleware" < spec.md
llmd task start a1b2c3d4e --assign claude-code

# Check progress
llmd agent runs --status running

# View the prompt template
llmd agent prompt claude-code developer

# Customise the auditor prompt for this project
llmd write agents/claude-code/auditor < custom-audit-template.md

# See all agent documents
llmd ls agents/
```

## Notes

- Agents run as subprocesses in git worktrees. Each task gets its
  own worktree for isolation.
- The agent process is responsible for moving the task when done.
  llmd records completion when the process exits.
- Worktrees are created at `.llmd/worktrees/<task-key>/` and cleaned
  up automatically after the agent exits.
- Built-in profiles: claude-code, gemini, aider. Use `--command` for
  anything else.

See `guide task` for task management and `guide workflow` for the
full task lifecycle with agents.
