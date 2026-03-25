# llmd agent

Manage AI agent configurations and runs. Agents are external tools
(Claude Code, Gemini CLI, Aider, etc.) that can be spawned to work
on tasks in isolated git worktrees.

Agent configurations and prompt templates are stored as documents
in the llmd store, making them versioned, portable, and editable.

## Usage

```
llmd agent add <name> <command> [args...]   Register an agent
llmd agent rm <name>                        Remove an agent
llmd agent ls                               List registered agents
llmd agent config <name>                    Show agent configuration
llmd agent prompt <name> <role>             Show prompt template
llmd agent runs [--status S] [--task K]     List agent runs
llmd agent stop <task-key>                  Stop a running agent
```

## Registering agents

Register an agent with its command and default arguments. The
`{{.Prompt}}` placeholder in arguments is replaced with the
assembled context prompt at spawn time.

```bash
# Claude Code
llmd agent add claude-code claude \
  -p "{{.Prompt}}" --output-format json

# Gemini CLI
llmd agent add gemini gemini \
  -p "{{.Prompt}}"

# With a role and budget
llmd agent add claude-auditor claude \
  -p "{{.Prompt}}" --output-format json \
  --role auditor --budget 1.00
```

### Add flags

| Flag | Description |
|------|-------------|
| `--role <role>` | Constrain agent to a role (developer, auditor) |
| `--budget <usd>` | Max spend per spawn in USD |

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

## Prompt templates

When an agent is registered, default prompt templates are seeded at:

```
agents/<name>/developer    Developer prompt template
agents/<name>/auditor      Auditor prompt template
```

View a template:

```bash
llmd agent prompt claude-code developer
```

Edit a template using standard document commands:

```bash
llmd edit agents/claude-code/developer "old text" "new text"
```

Or rewrite it entirely:

```bash
llmd write agents/claude-code/developer < my-template.md
```

### Template syntax

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

## Document storage

All agent configuration lives in the llmd store as documents:

```
agents/claude-code/config       JSON configuration
agents/claude-code/developer    Developer prompt template
agents/claude-code/auditor      Auditor prompt template
agents/gemini/config            JSON configuration
agents/gemini/auditor           Auditor prompt template
agents/default/developer        Shared default developer template
```

Because these are documents, they are versioned, searchable, and
travel with the repo. Use `llmd history agents/claude-code/config`
to see who changed the configuration and when.

## Environment variables

Agents receive these environment variables:

| Variable | Description |
|----------|-------------|
| `LLMD_TASK_ID` | The task key being worked on |
| `LLMD_AGENT` | The agent configuration name |

## Examples

```bash
# Set up two agents: one for development, one for auditing
llmd agent add claude-code claude \
  -p "{{.Prompt}}" --output-format json \
  --role developer --budget 3.00

llmd agent add gemini-auditor gemini \
  -p "{{.Prompt}}" \
  --role auditor --budget 1.00

# Create a task and start it with an agent
llmd task add "Implement auth middleware" < spec.md
llmd task start a1b2c3d4e --assign claude-code

# Check what's running
llmd agent runs --status running

# View the prompt the agent received
llmd agent prompt claude-code developer

# Customise the auditor template for this project
llmd write agents/gemini-auditor/auditor < custom-audit-template.md

# See all agent configuration
llmd ls agents/
```

## Notes

- Agents run as subprocesses in git worktrees. Each task gets its
  own worktree for isolation.
- The agent process is responsible for moving the task when done.
  llmd does not poll or monitor - it records completion when the
  process exits.
- Budget flags are platform-specific. Currently `--max-budget-usd`
  is appended for Claude Code agents.
- Worktrees are created at `.llmd/worktrees/<task-key>/` and cleaned
  up automatically after the agent exits.

See `guide task` for task management and `guide workflow` for the
full task lifecycle with agents.
