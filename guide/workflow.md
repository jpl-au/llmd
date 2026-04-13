# Workflow guide

Best practices for using llmd as a working document store with
agent orchestration.

## Set up your author

Every write records who made the change. Set your author name once:

```
llmd config author "Alice"
```

This writes to the local config at `.llmd/config.yaml`. To set it globally
(across all stores), use `--global`:

```
llmd config --global author "Alice"
```

AI agents must not use `config author` - pass `--author` on every
mutation command instead (see `guide`).

## Document path conventions

Paths use forward slashes and no leading slash. A path can be a
single word or a hierarchy - there is no requirement to use a prefix:

```
readme
todo
plan
notes/standup
projects/website/spec
meetings/2026-02-24
```

Single-segment paths like `readme`, `todo`, and `plan` are perfectly
valid and often the right choice for standalone documents. Only add
hierarchy when grouping genuinely related documents together.

Good conventions:
- Use the shortest path that makes sense - `todo` beats `docs/todo`
- Group related documents under a shared prefix when there are several
- Use dates for journals, meeting notes, and logs
- Keep paths short and descriptive

## Writing and iterating

Every `write` and `edit` creates a new version automatically. Use
`--message` to annotate why a change was made:

```
echo "First draft" | llmd write proposal
echo "Revised draft" | llmd write proposal --message "Added budget section"
llmd edit proposal "TBD" "Q3 2026" --message "Confirmed timeline"
```

## Reviewing changes

Check what changed and when:

```
llmd history proposal              # last 10 versions
llmd history -n5 proposal          # last 5 versions
llmd history --all proposal        # every version
llmd diff proposal                 # diff against previous version
llmd diff proposal:1 proposal:3   # diff between specific versions
llmd cat --version 2 proposal      # read an old version
```

## Using tags for workflow state

Tags are lightweight labels. Use them to track document state:

```
llmd tag proposal draft
llmd tag proposal review
llmd tag -d proposal draft              # remove the draft tag
llmd tag -f review                      # find all docs tagged "review"
```

## Linking related documents

Create directed links between documents:

```
llmd link meeting spec
llmd link --label "blocked-by" auth db-migration
```

## Search

Full-text search uses FTS5 syntax:

```
llmd grep budget                        # simple word search
llmd grep "budget AND timeline"         # boolean query
llmd grep budget projects/              # search within a prefix
```

## Task lifecycle

Tasks track work through columns on a board:

  backlog → up-next → in-progress → review → approval → done

Failed work goes to `blocked` for human intervention. These are the
default columns. Customise them with `task column add`, `task column
rm`, and `task column mv`.

### Creating tasks

Every task has a backing spec document that describes the work:

```
llmd task add "Fix auth tokens" <<'SPEC'
## Context

Auth tokens never expire, causing security issues.

## Acceptance Criteria

- Tokens expire after 1 hour
- Expired tokens return 401
SPEC
```

Tasks cannot leave the backlog until the spec has content beyond the
title heading (spec gating).

### Moving tasks through the board

```
llmd task move a1b2c3d4e up-next        # ready to start
llmd task move a1b2c3d4e in-progress    # claimed, work underway
llmd task move a1b2c3d4e review         # work done, ready for review
llmd task finish a1b2c3d4e              # approved, move to done
```

### The approval and blocked columns

- **approval** - agent completed successfully, waiting for human
  sign-off before done
- **blocked** - agent failed or got stuck, needs human intervention

These columns have no automation rules by default. A human reviews
the work (approval) or investigates the problem (blocked), then
moves the task forward or back.

## Agent orchestration

llmd can spawn AI agents to work on tasks automatically. The full
pipeline: register agents, configure rules, create tasks, and let
the system drive the workflow.

### 1. Register agents

```
llmd agent add claude-code
llmd agent add gemini
```

Agent configurations, prompt templates, and settings are stored as
plain files in `.llmd/agents/`. Edit them directly with any editor.

### 2. Configure rules

Rules define what happens when a task enters a column. View the
defaults:

```
llmd rule list
```

Automate columns by assigning agents:

```
llmd rule set in-progress --agent claude-code --role developer
llmd rule set review --agent gemini --role auditor
```

Each rule shows its transitions:

```
in-progress [claude-code, developer]
 ├─ success: review →
 └─ failure: blocked ←
```

See `guide rule` for full details.

### 3. Create a task and start the pipeline

```
llmd task add "Fix auth tokens" < spec.md
llmd task move <key> in-progress
```

From here, automation takes over:

1. claude-code spawns, implements the spec
2. Task moves to review on success
3. gemini spawns, audits the code
4. If approved: task moves to approval (human sign-off)
5. If rejected: task moves to blocked (human investigates)

### 4. Monitor progress

```
llmd task board                         # board view
llmd agent runs                         # agent run status
llmd agent runs --status running        # active agents
llmd task log <key>                     # audit trail for a task
```

### 5. Human checkpoints

When a task reaches **approval**, review the work and move to done:

```
llmd task finish <key>
```

When a task is in **blocked**, investigate and retry:

```
llmd audit list <key>                   # see what went wrong
llmd task move <key> in-progress        # retry with developer
```

## Audit threads for review feedback

Audits are the feedback loop between contributors and reviewers.
They are immutable, insert-only threads attached to a task or
document.

```
# Reviewer flags an issue
llmd --author "alice" audit add a1b2c3d4e "Error handling missing" \
  --assign bob

# Coder checks their inbox and responds
llmd --author "bob" audit status
llmd --author "bob" audit reply <audit-id> "Fixed in latest commit"

# Reviewer approves the thread
llmd --author "alice" audit resolve <audit-id>
```

See `guide audit` for full details.

## Git integration

Link tasks to git branches for traceability:

```
llmd task start a1b2c3d4e               # records branch, moves to in-progress
llmd task diff a1b2c3d4e                # diff against default branch
llmd task files a1b2c3d4e               # list changed files
```

When agents are spawned, they work in isolated git worktrees on
dedicated branches. The worktree persists across pipeline steps
(developer, tester, auditor) and is cleaned up by `task finish`.

## Multi-agent collaboration

- **The board is the source of truth** for what needs doing, what's
  in progress, and what's waiting for review.
- **Rules drive the pipeline.** Configure which agent handles each
  column and where tasks go on success or failure.
- **Audits are the communication channel.** Agents leave audit
  trails explaining decisions, flagging issues, and confirming
  approvals.
- **Author attribution matters.** Every mutation carries an author,
  so the history shows who did what.
