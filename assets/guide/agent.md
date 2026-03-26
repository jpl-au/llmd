# llmd agent

Manage AI agent configurations and runs. Agents are external tools
(Claude Code, Gemini CLI, Aider, etc.) that can be spawned to work
on tasks in isolated git worktrees.

Agent configurations, prompt templates, and runtime settings are
stored as plain files in `.llmd/agents/`, making them immediately
discoverable and editable with any text editor.

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

## Registering agents

llmd ships with built-in profiles for common agents. Registration
is a single command:

```bash
llmd agent add claude-code
llmd agent add gemini
llmd agent add aider
```

That's it. llmd knows the binary, arguments, and prompt patterns
for each. Default prompt templates and runtime settings are seeded
automatically.

### Custom agents

For agents llmd doesn't know about, provide the command:

```bash
llmd agent add my-agent --command ./my-tool
```

### What happens on add

When you register an agent, llmd creates files on disk:

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

View the configuration:

```bash
llmd agent config claude-code
llmd agent prompt claude-code developer
```

Edit files directly:

```bash
vi .llmd/agents/claude-code/developer.md
```

## Spawning agents

There are two ways to spawn an agent:

```bash
# Via agent spawn (natural shorthand)
llmd agent spawn <task-key> claude-code

# Via task start with --assign
llmd task start <task-key> --assign claude-code
```

Both do the same thing:

1. Check that the task's dependencies are satisfied
2. Create a git branch if the task doesn't have one
3. Create an isolated git worktree at `.llmd/worktrees/<task-key>/`
4. Assemble the context prompt from the task's spec and templates
5. Write a run config (`.llmd-run.json`) to the worktree
6. Start `llmd agent run` as a detached process
7. Record the run in the agent_activity table

The `agent run` process then starts the actual agent (claude, gemini,
etc.), waits for it to finish, records completion stats, captures
failure output as an audit note, and moves the task based on the
exit code and rule transitions.

### Automatic spawning via rules

When a column has a rule with an agent configured, the agent is
spawned automatically when a task enters that column. See
`guide rule` for details.

## Agent roles

Agents can fill three roles:

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

### Fallback chain

When spawning, the prompt template is resolved in order:

1. `.llmd/agents/<name>/<role>.md` - agent-specific template
2. `.llmd/agents/default/<role>.md` - shared default template
3. Built-in embedded template

If the HTTP server is running, the `-http` variant is tried first
(e.g. `developer-http.md` before `developer.md`).

## Run tracking

Every agent spawn is recorded in the `agent_activity` table with:

- Monetary cost (when reported by the agent)
- Input and output token counts
- Model name
- Exit code, start/stop timestamps
- Git branch and worktree path

```bash
# List all runs
llmd agent runs

# Filter by status
llmd agent runs --status running

# Filter by task
llmd agent runs --task a1b2c3d4e
```

### Run statuses

| Status | Meaning |
|--------|---------|
| `running` | Agent process is active |
| `completed` | Agent exited successfully (exit code 0) |
| `failed` | Agent exited with an error |
| `stopped` | Agent was manually terminated |

## Completion

When an agent process exits, `llmd agent run` records the result
automatically. It extracts stats (cost, tokens, model) from the
agent's output log and updates the run record.

On failure, the last output from the agent log is captured as an
audit note on the task so humans can see what went wrong.

The task is then moved to the next column based on the exit code
and the rule's success/failure transitions.

## Worktree lifecycle

Worktrees persist across the full pipeline. When a developer agent
finishes and the task moves to testing, the tester agent uses the
same worktree and branch. The worktree is only cleaned up when
`task finish` is called (moving the task to done).

## Platform support

llmd detects the agent platform and handles differences
automatically:

| Platform | Budget flags | Cost extraction | Settings location |
|----------|-------------|-----------------|-------------------|
| Claude Code | `--max-budget-usd` | JSON output (`total_cost_usd`) | `.claude/settings.json` |
| Gemini CLI | - | Token counts from JSON | - |
| Generic | - | - | - |

## Examples

```bash
# Register agents
llmd agent add claude-code
llmd agent add gemini

# Set up automated pipeline via rules
llmd rule set code --agent claude-code --role developer
llmd rule set review --agent gemini --role auditor

# Create a task and kick off the pipeline
llmd task add "Fix auth bug" < spec.md
llmd task move <key> code
# Automation takes over from here

# Monitor progress
llmd agent runs --status running
llmd task board
```

See `guide rule` for column automation and `guide task` for task
management.
