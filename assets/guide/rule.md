# llmd rule

Rules define what happens when a task enters a board column. Each
column can have a rule that specifies where the task goes on success
or failure, and optionally which agent handles the work automatically.

Rules are stored as YAML files in `.llmd/rules/`. The default rule
set (`default.yaml`) is created when you run `llmd init`.

## Usage

```
llmd rule show                     Display all column rules
llmd rule set <column> [flags]     Configure a column rule
llmd rule unset <column>           Remove agent (keep transitions)
```

## How rules work

A rule has four fields:

| Field | Description |
|-------|-------------|
| `success` | Column to move to when the agent exits successfully |
| `failure` | Column to move to when the agent exits with an error |
| `agent` | Agent to auto-spawn when a task enters this column (optional) |
| `role` | Role for the agent: developer, tester, or auditor (optional) |

Columns without an `agent` field are **manual** - a human (or an
explicit `--assign` flag) triggers the work. The transitions still
apply: the wrapper script reads success/failure from the rule
regardless of whether the column is manual or automated.

## Default rules

After `llmd init`, the default rule set looks like this:

```yaml
# .llmd/rules/default.yaml
in-progress:
  success: review
  failure: in-progress
review:
  success: done
  failure: in-progress
up-next:
  success: in-progress
  failure: up-next
```

All columns are manual by default. The board columns are:
backlog, up-next, in-progress, review, done.

Columns not listed (backlog, up-next, done) have no transitions -
tasks stay there until moved manually.

## Automating columns

Add an agent to a column to make it automated:

```bash
llmd rule set code --agent claude-code --role developer
```

Now when a task enters the "code" column, claude-code is
automatically spawned to work on it. When claude finishes
successfully, the wrapper moves the task to "test" (the success
transition). If it fails, the task goes to "blocked".

## Setting up a full pipeline

```bash
# Developer writes code
llmd rule set code --agent claude-code --role developer

# Tester verifies the implementation
llmd rule set test --agent claude-code --role tester

# Auditor reviews the code
llmd rule set review --agent gemini --role auditor
```

The pipeline emerges from the column order and their individual
rules. Each column is independent - it only knows its own agent
and transitions.

## The pipeline in action

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

## Review loops

When an auditor rejects work, the task moves back to an earlier
column (defined by the review column's failure transition). If that
column has an agent configured, the agent is automatically
re-spawned with the audit feedback in context. This creates a
natural review loop until the work is approved.

## Manual columns

Columns without a rule (or without an agent in their rule) are
manual stopping points. Use them for:

- **Human review**: a "ready-for-pr" column with no agent where a
  human checks the work before merging
- **Blocked work**: a "blocked" column where tasks wait for human
  intervention
- **Triage**: the "backlog" and "up-next" columns where humans
  decide what to work on

## Editing rules directly

Rules are plain YAML files. You can edit them with any text editor:

```bash
vi .llmd/rules/default.yaml
```

Changes take effect immediately - rules are read from disk on every
task move.

## Rule set flags

| Flag | Description |
|------|-------------|
| `--agent <name>` | Agent to auto-spawn (must be registered) |
| `--role <role>` | Agent role: developer, tester, or auditor |
| `--success <column>` | Column to move to on success |
| `--failure <column>` | Column to move to on failure |

When using `rule set`, only the flags you provide are updated. Existing
values are preserved. This lets you change just the agent without
affecting the transitions:

```bash
# Change agent but keep existing success/failure transitions
llmd rule set code --agent gemini
```

## Removing automation

```bash
llmd rule unset code
```

This removes the agent and role from the column's rule but keeps the
success/failure transitions intact. The column becomes manual again.

## Notes

- Rules are stored in `.llmd/rules/default.yaml` and committed with
  the repository
- The wrapper script reads transitions from the rule at spawn time
- Agent prompt templates can reference `{{.OnSuccess}}` and
  `{{.OnFailure}}` to tell the agent which columns to use
- Worktrees persist across the full pipeline and are cleaned up by
  `task finish`, not when individual agents complete

See `guide agent` for agent registration and `guide task` for task
management.
