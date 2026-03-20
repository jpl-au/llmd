# llmd-Orchestrated Agent System Specification

## Goal

Build a lightweight orchestration layer around **llmd** that coordinates multiple
AI agents working on tasks. llmd is the single source of truth - specs, tasks,
audits, documents, links, and the message queue all live there. The orchestrator
watches task status changes and spawns the right agent for the job.

Agents are short-lived and single-purpose. They get spawned with context, do their
work, move the task to the next status, and exit. They interact with llmd through
whatever transport suits them (CLI, HTTP, MCP). The orchestrator doesn't care how
they talk to llmd - only that the task status changes when they're done.

**See also:** `hooks-audit.md` for the agent hook survey and framework-agnostic
integration patterns.

---

## How It Works

### The Human/Agent Workflow

1. **Specs get written.** A human works with an agent (or alone) to create
   specification documents in llmd. These are versioned markdown.

2. **Tasks get created.** From specs, tasks are created and linked to them.
   Tasks live on a board with columns (backlog, todo, in-progress, review, done,
   failed). Tasks can be assigned to specific agents and flagged (e.g. "needs-audit").

3. **Orchestration starts.** The human (or agent) kicks off the orchestration
   process. The orchestrator watches the task board for status changes.

4. **Agents get spawned.** The orchestrator sees a task in "todo", spawns the
   right agent for the job (Claude for development, Gemini for auditing, etc.),
   gives it the task context (spec, audit history, linked docs), and lets it run.

5. **Agent does work, moves task, exits.** The agent reads the spec, writes code,
   updates docs - whatever the task requires, using whatever llmd features it
   needs through whatever transport it has. When done, it moves the task to the
   next column and exits.

6. **Orchestrator reacts.** The status change triggers the next step - maybe an
   auditor needs to review, maybe a dependent task is now unblocked, maybe the
   human gets notified.

7. **Repeat until done.**

### The Task Board as State Machine

The orchestrator is a **reactor**. It watches `task.moved` events and maps them
to actions:

```
task.moved → "todo"          → check dependencies, spawn developer if ready
task.moved → "in-progress"   → (agent working, nothing to do)
task.moved → "review"        → spawn auditor
task.moved → "done"          → check dependents, notify human, create follow-ups
task.moved → "failed"        → alert human, maybe retry
```

When a task moves from "review" back to "in-progress" (auditor found issues),
the orchestrator spawns a fresh developer agent with the audit feedback as
context. The developer fixes the issues, moves back to "review", and the cycle
repeats until the auditor approves and moves to "done".

```
┌──────┐    spawn     ┌───────────┐   move to    ┌────────┐
│ todo │───developer──►│in-progress│───"review"──►│ review │
└──────┘              └───────────┘              └───┬────┘
                           ▲                         │
                           │    move to              │ spawn
                           │  "in-progress"          │ auditor
                           │  (with feedback)        │
                           │                         ▼
                           │                    ┌─────────┐
                           └────────────────────│ auditor │
                                needs work      │ decides │
                                                └────┬────┘
                                                     │
                                              approved│
                                                     ▼
                                                 ┌──────┐
                                                 │ done │
                                                 └──────┘
```

Agents don't talk to each other. They don't need to know other agents exist.
They coordinate through the task board - the status is the protocol.

---

## Two Modes of Operation

### 1. Orchestrator-Spawned (headless)

Short-lived, single task, exits when done. The orchestrator manages the full
lifecycle.

- Agent gets spawned with `-p` (non-interactive) and a prompt containing the
  task context
- Does the work, moves the task, exits
- No polling needed - the assignment is in the prompt
- Fresh context every time (no session bloat)
- The orchestrator detects the status change and decides what's next

This is the primary mode for automated workflows.

### 2. Long-Running / Interactive

A Claude session (or similar) that stays alive - either paired with a human or
running autonomously. This agent participates in the workflow by polling the
queue or receiving queue state via hooks.

- `SessionStart` hook injects pending queue messages into context
- Agent can check the queue via MCP tools or CLI during the session
- Picks up messages ("task X assigned to you", "audit Y needs your review"),
  acts on them, acks them
- Can coexist with orchestrator-spawned agents - a human might be pairing with
  Claude while Gemini auditors are being spawned in the background

The queue is how this mode discovers work. The task board is still the state
machine - the long-running agent reads tasks and moves them just like a
spawned agent does.

---

## Core Components

### 1. llmd (central store)

Already built. Provides everything the orchestrator and agents need:

| Concern | llmd feature | How agents use it |
|---------|-------------|-------------------|
| Specs | `DocumentStore` - versioned markdown | Read specs, write results |
| Tasks | `TaskStore` - board with columns, assignment, flags | Read task, move status, update metadata |
| Dependencies | `LinkStore` - directed links with labels | Orchestrator checks "depends-on" links |
| Communication | `QueueStore` - durable, ordered, per-consumer | Long-running agents poll for work |
| Reviews | `AuditStore` - threaded reviews with status | Auditors review, reply, resolve |
| Real-time events | SSE (`GET /events`) + webhooks | Orchestrator watches task.moved |
| Git integration | `TaskStore.Start/StartBranch/Finish/ByBranch` | Branch per task, diff on completion |
| Activity timeline | `ActivityStore.Recent` | Cross-domain history |

**Transport is the agent's choice.** An agent connected via MCP uses MCP tools.
An agent with HTTP access uses `llmd serve`. A shell hook calls the CLI. The
orchestrator doesn't care - it watches the task board state, not how it changed.

### 2. Go Orchestrator

A thin Go daemon. It does three things:

1. **Watch** - subscribe to `task.moved` events via SSE (fall back to polling)
2. **Decide** - map (new status, task metadata) → action
3. **Spawn** - launch the right agent with the right context

```go
// Subscribe to task events
events := subscribeSSE("http://localhost:5563/events?type=task.moved")

for event := range events {
    task, _ := ctx.Tasks.Read(event.Key)

    switch task.Status {
    case "todo":
        if ready(ctx, task) {
            spawn(ctx, task, "developer")
        }
    case "review":
        spawn(ctx, task, "auditor")
    case "in-progress":
        // If it came from "review", a developer is being re-spawned
        // with audit feedback. The spawn function handles context.
        if cameFromReview(event) {
            spawn(ctx, task, "developer")
        }
    case "done":
        notifyHuman(ctx, task)
        unblockDependents(ctx, task)
    case "failed":
        alertHuman(ctx, task)
    }
}
```

The orchestrator uses the Go SDK directly (in-process access to SQLite). It
doesn't need HTTP or CLI - it imports the SDK packages.

### 3. Agents

Agents are subprocesses. Each platform has its own invocation:

| Role | Platform | Invocation |
|------|----------|------------|
| Developer | Claude Code | `claude -p "..." --max-turns 50 --output-format json` |
| Auditor | Gemini CLI | `gemini -p "..."` |
| Developer | Any agent | Wrapper script |
| Auditor | Any agent | Wrapper script |

The role→platform mapping is configurable. You might use Claude for both
development and auditing, or Gemini for both, or mix them.

**Budget control:** `--max-budget-usd N` (Claude Code) or equivalent per spawn.

### 4. Context Assembly

The orchestrator builds the prompt from llmd data. What goes in depends on the
role and the task state:

**Developer (fresh task):**
- Task ID, title, assignment
- Spec document content (read from linked doc)
- Linked reference documents
- Git branch to work in

**Developer (back from review):**
- Everything above, plus:
- Audit thread content (the auditor's feedback)
- Diff of what was changed in the previous attempt

**Auditor:**
- Task ID, title, spec document
- Git diff of changes made by the developer
- Audit instructions (what to check)
- Previous audit thread (if this is a re-review)

```go
func buildPrompt(ctx sdk.Context, task *sdk.Task, role string) string {
    spec, _ := ctx.Documents.Read(task.Path, 0)
    audits, _ := ctx.Audits.List(sdk.AuditListOpts{Target: task.Key})

    var b strings.Builder
    fmt.Fprintf(&b, "Task: %s\nTitle: %s\nBranch: %s\n\n", task.Key, task.Title, task.Branch)
    fmt.Fprintf(&b, "## Spec\n\n%s\n\n", spec)

    if len(audits) > 0 {
        fmt.Fprintf(&b, "## Audit History\n\n")
        for _, a := range audits {
            fmt.Fprintf(&b, "**%s** (%s): %s\n\n", a.Author, a.Status, a.Content)
        }
    }

    switch role {
    case "developer":
        fmt.Fprintf(&b, "## Instructions\n\n")
        fmt.Fprintf(&b, "- Work in git branch %s\n", task.Branch)
        fmt.Fprintf(&b, "- Implement the spec above\n")
        if len(audits) > 0 {
            fmt.Fprintf(&b, "- Address the audit feedback above\n")
        }
        fmt.Fprintf(&b, "- When done, move the task to 'review'\n")

    case "auditor":
        diff, _, _, _ := ctx.Documents.Diff(task.Branch, "main", 3)
        fmt.Fprintf(&b, "## Changes to Review\n\n```\n%s\n```\n\n", diff)
        fmt.Fprintf(&b, "## Instructions\n\n")
        fmt.Fprintf(&b, "- Review changes against the spec\n")
        fmt.Fprintf(&b, "- If approved: move task to 'done'\n")
        fmt.Fprintf(&b, "- If issues found: move task to 'in-progress' and create an audit reply explaining what needs fixing\n")
    }

    return b.String()
}
```

---

## Task ID Propagation

The task ID flows from orchestrator → agent → llmd. Three mechanisms, all set
before spawning:

1. **Environment variable** - `LLMD_TASK_ID` on the subprocess
2. **File marker** - `.llmd-task` in the working directory
3. **Prompt** - embedded in the `-p` text

For spawned agents, the task ID is in the prompt. For long-running agents
checking the queue, the task ID comes from the queue message payload.

---

## The Queue's Role

The queue is **not** the orchestration mechanism - the task board is. The queue
serves a different purpose:

**What the queue is for:**
- Messages between humans and agents ("please prioritise task X")
- Notifications that don't map to task status ("spec Y was updated")
- Inbox for long-running agents that poll for work
- Status updates directed at specific consumers ("task X completed" → human)
- Domain events that agents might care about (document changes, tag additions)

**What the task board is for:**
- Orchestration state machine (todo → in-progress → review → done)
- Triggering agent spawns (orchestrator watches `task.moved`)
- Dependency tracking (links between tasks)
- Assignment (who's responsible)

The queue accumulates between polls. The task board is the real-time state.
Both are needed - the queue handles the messaging that doesn't fit into a
status column.

### Queue for Long-Running Agents

A Claude session that stays alive reads the queue on startup and at natural
hook points:

```json
{
  "hooks": {
    "SessionStart": [{
      "matcher": "startup",
      "hooks": [{
        "type": "command",
        "command": "llmd --author claude-code queue ls --limit 10"
      }]
    }]
  }
}
```

Or via MCP tools mid-session. The agent sees messages like:

```
[a8f3k2m1n] task.created - "Implement auth middleware" (assigned to you)
[b7e2j4n9p] direct - "Please prioritise the auth work before the API changes" (from jake)
```

It acts on them, acks them, and continues.

---

## Dependency Handling

Dependencies are **directed links** between task spec documents:

```bash
# Task B depends on Task A
llmd --author jake link docs/specs/task-B docs/specs/task-A "depends-on"
```

The orchestrator checks before spawning:

```go
func ready(ctx sdk.Context, task *sdk.Task) bool {
    links, _ := ctx.Links.List(task.Path, "out")
    for _, link := range links {
        if link.Label != "depends-on" {
            continue
        }
        // Reverse lookup: find task that owns this spec
        depTasks, _ := ctx.Tasks.List(sdk.TaskListOpts{})
        for _, dt := range depTasks {
            if dt.Path == link.To && dt.Status != "done" {
                return false
            }
        }
    }
    return true
}
```

**Future improvement:** `TaskStore.ByPath(path)` for O(1) reverse lookup.

---

## Audit / Review Flow

Auditing is built into the state machine, not bolted on.

1. Developer finishes work, moves task to "review"
2. Orchestrator sees `task.moved → review`, spawns auditor with diff + spec
3. Auditor reviews. Two outcomes:
   - **Approved:** auditor calls `audit resolve`, moves task to "done"
   - **Needs work:** auditor calls `audit reply --status needs-work` with
     feedback, moves task back to "in-progress"
4. If back to "in-progress": orchestrator spawns fresh developer with audit
   feedback included in context
5. Cycle repeats until approved

**Depth limit:** The orchestrator tracks review cycles per task. After N
round-trips (configurable, default 3), it escalates to the human rather
than spawning another attempt.

---

## Git Branch Strategy

llmd tracks branches per task. The orchestrator uses this for isolation:

- **Single agent:** `TaskStore.StartBranch` creates a branch from the task title
- **Parallel agents:** each gets a git worktree so they don't conflict
- **On completion:** `TaskStore.Finish` captures the git diff summary

```go
// Create worktree for isolation
worktree := filepath.Join(worktreeDir, task.Key)
exec.Command("git", "worktree", "add", worktree, task.Branch).Run()

// Spawn agent in the worktree
cmd := exec.CommandContext(ctx, agentCommand, agentArgs...)
cmd.Dir = worktree

// Clean up after completion
exec.Command("git", "worktree", "remove", worktree).Run()
```

---

## Compaction Resilience

For long-running Claude sessions, context compaction can lose state. Mitigations:

1. **Post-compaction hook** re-injects queue state and current task context
2. **Short tasks by design** - orchestrator-spawned agents get fresh context
   per task, so compaction is irrelevant for mode 1

---

## Non-Functional Requirements

- **Parallelism:** goroutines + semaphore (configurable, default 4 concurrent)
- **Logging:** structured slog (task_id, agent, role, status, duration)
- **Error handling:** mark "failed" on timeout/crash, retry with backoff,
  escalate to human after N failures
- **Cost control:**
  - Per-task: `--max-budget-usd N` per spawn
  - Global: aggregate ceiling that pauses scheduling
- **Graceful shutdown:** drain running agents, let them finish or move tasks
  back to previous status
- **Idempotency:** on restart, scan for "in-progress" tasks without running
  processes and reset them

---

## Configuration

```yaml
# docs/orchestrator/config.yaml (stored in llmd)
poll_interval: 5s          # fallback if SSE unavailable
max_concurrent: 4
budget_per_task: 3.00
budget_global: 50.00
max_review_cycles: 3       # escalate to human after this many round-trips
retry_limit: 2

roles:
  developer:
    agent: claude-code
    command: claude
    args: ["-p", "{{.Prompt}}", "--max-turns", "50", "--output-format", "json"]
    budget: 3.00
  auditor:
    agent: gemini
    command: gemini
    args: ["-p", "{{.Prompt}}"]
    budget: 1.00

# Status → role mapping (the state machine)
transitions:
  todo: developer
  review: auditor
  # "in-progress" from "review" → developer (with audit context)
```

---

## Implementation Order

1. **Watch loop** - subscribe to `task.moved` via SSE, log transitions
2. **Single spawn** - spawn one Claude for a "todo" task, verify it moves to "review"
3. **Auditor spawn** - on "review", spawn Gemini, verify it moves to "done" or back
4. **Round-trip** - test the full developer → auditor → developer → done cycle
5. **Dependencies** - link-based dependency checking before spawn
6. **Parallelism** - goroutine pool, worktrees, multiple concurrent tasks
7. **Queue integration** - long-running agent mode, human notifications
8. **Cost control** - budgets, spend tracking, global ceiling
9. **Escalation** - review cycle limits, failure handling, human alerts

---

## Open Questions

- **Task metadata:** `Task.Flags` is comma-separated strings today. Is this
  sufficient for orchestration config (retry count, model preference, timeout)?
  Or should tasks gain structured metadata?

- **Spec → task reverse lookup:** dependency checking needs to go from spec
  path → owning task. Currently requires scanning all tasks. `TaskStore.ByPath`
  would make this efficient.

- **Worktree lifecycle:** orchestrator creates them, but what if it crashes
  mid-task? Periodic cleanup needed.

- **Transcript storage:** hooks expose `transcript_path`. Worth ingesting
  into llmd as documents for post-hoc auditing? Summaries might be more
  practical than full transcripts.

- **Mixed orchestration:** can a human manually move a task to "review" and
  have the orchestrator spawn an auditor? Probably yes - the orchestrator
  reacts to status changes regardless of who caused them. But worth validating.
