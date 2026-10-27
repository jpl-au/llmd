# llmd rule

Say what happens when a task enters a column: where it goes on success
or failure, and which agent, if any, runs it.

## Usage

```
llmd rule list                     Display all column rules (aliases: ls, show)
llmd rule set <column> [flags]     Configure a column rule
llmd rule unset <column>           Remove agent (keep transitions)
```

## Flags

| Flag | Subcommands | Description |
|------|-------------|-------------|
| `--agent <name>` | set | Agent to auto-spawn (must be registered) |
| `--role <role>` | set | Agent role: developer, tester, or auditor |
| `--success <column>` | set | Column to move to on success |
| `--failure <column>` | set | Column to move to on failure |
| `--resume` | set | Resume previous session when auto-spawning |

## Rules

- `set` and `unset` change files under `.llmd/rules/`; `list` reads
  them. Rules are read from disk on every task move, so an edit takes
  effect at once.
- `set` updates only the flags you pass and keeps the rest. `unset`
  removes the agent and role but keeps the success and failure
  columns.
- A column with an agent is automated: the agent is spawned when a
  task enters it. A column without one is manual: a person, or
  `task start --agent`, triggers the work. Transitions apply either
  way.
- The agent named in `--agent` must be registered (`guide agent`).
- Columns with no rule (backlog, up-next and done by default) have no
  transitions: tasks stay until moved by hand.
- `list` prints a styled tree of each column's agent and transitions;
  `--json` gives the structured form.

## Examples

```bash
# Automate a column
llmd --author alice rule set code --agent claude-code --role developer

# A full pipeline: developer, tester, auditor
llmd --author alice rule set code --agent claude-code --role developer
llmd --author alice rule set test --agent claude-code --role tester
llmd --author alice rule set review --agent gemini --role auditor

# Change the agent but keep the existing success/failure transitions
llmd --author alice rule set code --agent gemini

# Let a re-spawned agent continue its previous session
llmd --author alice rule set code --agent claude-code --role developer --resume

# Make a column manual again
llmd --author alice rule unset code

# See every rule
llmd rule list
```

## How rules work

A rule has five fields:

| Field | Description |
|-------|-------------|
| `success` | Column to move to when the agent exits successfully |
| `failure` | Column to move to when the agent exits with an error |
| `agent` | Agent to auto-spawn when a task enters this column (optional) |
| `role` | Role for the agent: developer, tester, or auditor (optional) |
| `resume` | Resume the previous agent's session context (optional, default false) |

Columns without an `agent` field are **manual** - a human (or an
explicit `task start --agent`) triggers the work. The transitions still
apply: `llmd agent run` reads success/failure from the rule
regardless of whether the column is manual or automated.

### Default rules

After `llmd init`, the default rule set looks like this:

```yaml
# .llmd/rules/default.yaml
in-progress:
  success: review
  failure: blocked
review:
  success: approval
  failure: blocked
up-next:
  success: in-progress
  failure: up-next
```

All columns are manual by default. The board columns are:
backlog, up-next, in-progress, review, approval, done, blocked.

The `approval` column is a positive human checkpoint - the agent
completed its work and a human signs off before done. The `blocked`
column is the escape hatch - the agent failed and a human needs to
investigate before retrying.

Columns not listed (backlog, up-next, done) have no transitions -
tasks stay there until moved manually.

### Editing rules directly

Rules are plain YAML files. You can edit them with any text editor:

```bash
vi .llmd/rules/default.yaml
```

Changes take effect immediately - rules are read from disk on every
task move.

## Automating columns

Add an agent to a column to make it automated:

```bash
llmd rule set code --agent claude-code --role developer
```

Now when a task enters the "code" column, claude-code is
automatically spawned to work on it. When claude finishes
successfully, the task moves to "test" (the success transition).
If it fails, the task goes to "blocked".

### The pipeline in action

```bash
# Create a task with a spec
llmd task add "Fix auth bug" < spec.md

# Move it to the first automated column
llmd task move <key> code

# From here, automation takes over:
# 1. claude-code spawns, writes the fix
# 2. Task moves to "test" on success
# 3. claude-code spawns as tester, runs tests
# 4. Task moves to "review" on success
# 5. gemini spawns as auditor, reviews the code
# 6. If approved: task moves to "done"
# 7. If rejected: task moves back to "code", claude re-spawns
```

The pipeline emerges from the column order and their individual
rules. Each column is independent - it only knows its own agent
and transitions.

## Review loops

When an auditor rejects work, the task moves back to an earlier
column (defined by the review column's failure transition). If that
column has an agent configured, the agent is automatically
re-spawned with the audit feedback in context. This creates a
natural review loop until the work is approved.

### Resuming context across review loops

By default, a re-spawned agent starts with a fresh prompt and no
memory of its previous work. Set `resume: true` on the column rule
to let the agent pick up its previous conversation context:

```bash
llmd rule set code --agent claude-code --role developer --resume
```

Or in YAML:

```yaml
code:
  agent: claude-code
  role: developer
  resume: true
  success: test
  failure: blocked
```

When a task loops back to this column and the previous run was by
the same agent, the agent is resumed with its full session history
instead of starting cold. This is valuable because the agent already
understands the codebase context, what it implemented, and why. The
audit feedback is provided as a new message in the existing
conversation rather than needing to be re-contextualised from
scratch.

Resume is best-effort: if the previous run was by a different agent,
produced no session ID, or the platform does not support session
resumption, a fresh prompt is assembled automatically.

Resume can also be set per-task via the task flag (`llmd task set
<key> --flag resume`) which applies regardless of the column rule.
The task flag and the column rule compose: either being true triggers
a resume attempt.

## Manual columns

Columns without a rule (or without an agent in their rule) are
manual stopping points. Use them for:

- **Human review**: a "ready-for-pr" column with no agent where a
  human checks the work before merging
- **Blocked work**: a "blocked" column where tasks wait for human
  intervention
- **Triage**: the "backlog" and "up-next" columns where humans
  decide what to work on

## Notes

- Rules are stored in `.llmd/rules/default.yaml` and committed with
  the repository
- Transitions are resolved from the rule at spawn time
- Agent prompt templates can reference `{{.OnSuccess}}` and
  `{{.OnFailure}}` to tell the agent which columns to use
- Worktrees persist across the full pipeline and are cleaned up by
  `task finish`, not when individual agents complete

See `guide agent` for agent registration and `guide task` for task
management.
