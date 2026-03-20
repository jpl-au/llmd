# Task Dependencies - Design

## Goal

Add dependency tracking to the task system. A task can depend on **one** other
task. Dependencies form simple chains: A depends on B, B depends on C. The full
chain is discovered by walking backwards - read A, see it depends on B, read B,
see it depends on C, read C, no dependency, done.

## Storage

Add a nullable `depends_on` column to the tasks table:

```sql
ALTER TABLE tasks ADD COLUMN depends_on TEXT;  -- single task key or NULL
```

- `NULL` - no dependency
- `"a8f3k2m1n"` - depends on this task

Plain text column. No JSON, no arrays. One task key or nothing.

```sql
CREATE INDEX idx_tasks_depends_on ON tasks(depends_on) WHERE depends_on IS NOT NULL;
```

The index supports both directions efficiently:
- Forward: "what does task X depend on?" → read `depends_on` from the row
- Reverse: "who depends on task X?" → `WHERE depends_on = ?` (indexed)

## SDK Interface

New methods on `TaskStore`:

```go
// Dep returns the single task this task depends on, or nil if none.
Dep(key string) (*Task, error)

// Dependents returns tasks that directly depend on this task (reverse).
Dependents(key string) ([]*Task, error)

// Chain returns the full dependency chain starting from key, walked
// backwards until a task with no dependency is reached. Returns tasks
// in dependency order (deepest dependency first, key last).
Chain(key string) ([]*Task, error)

// Ready returns true if the full dependency chain is satisfied
// (every task in the chain has status "done", or there are no deps).
Ready(key string) (bool, error)
```

### Modifications to existing methods

- `TaskAddOpts` gains `DependsOn string` - set the dependency at creation
- `TaskSetOpts` gains `DependsOn *string` - pointer semantics: nil means
  don't change, pointer to empty string clears the dependency
- `Task` model gains `DependsOn string` - read from the column directly

## CLI

```bash
# Add task with dependency
llmd --author jake task add "Build auth middleware" --depends-on a8f3k2m1n

# Set dependency on existing task
llmd --author jake task set a8f3k2m1n --depends-on b7e2j4n9p

# Clear dependency
llmd --author jake task set a8f3k2m1n --depends-on ""

# View dependency chain
llmd task chain a8f3k2m1n

# Check if task is ready
llmd task ready a8f3k2m1n
```

### `task chain` output

```
a8f3k2m1n  Build auth middleware           [todo]
└─ b7e2j4n9p  Design auth spec             [done] ✓
   └─ k1m2n3p4q  Create base schema        [done] ✓
```

A linear chain. Each task has at most one child in the tree. Easy to read,
easy to reason about.

### `task list` modifications

Show the dependency key inline when present:

```
KEY        STATUS    TITLE                         DEPENDS ON
a8f3k2m1n  todo      Build auth middleware          b7e2j4n9p
b7e2j4n9p  done      Design auth spec              k1m2n3p4q
k1m2n3p4q  done      Create base schema            -
```

## Queries

### Walk the chain (Go, not SQL)

The chain is simple enough to walk in Go - no recursive CTE needed:

```go
func (s *Store) Chain(key string) ([]*Task, error) {
    var chain []*Task
    seen := map[string]bool{}

    for {
        t, err := s.Read(key)
        if err != nil {
            return nil, err
        }
        chain = append(chain, t)
        seen[key] = true

        if t.DependsOn == "" {
            break
        }
        if seen[t.DependsOn] {
            return nil, fmt.Errorf("cycle detected: %s", t.DependsOn)
        }
        key = t.DependsOn
    }

    // Reverse so deepest dependency is first
    slices.Reverse(chain)
    return chain, nil
}
```

Each iteration is a single row read by primary key. A chain of 10 tasks
is 10 queries - trivial.

### Readiness check

```go
func (s *Store) Ready(key string) (bool, error) {
    t, err := s.Read(key)
    if err != nil {
        return false, err
    }

    for t.DependsOn != "" {
        dep, err := s.Read(t.DependsOn)
        if err != nil {
            return false, err
        }
        if dep.Status != "done" {
            return false, nil
        }
        t = dep
    }

    return true, nil
}
```

Walk the chain. If any task isn't done, not ready.

### Reverse lookup

```sql
SELECT * FROM tasks WHERE depends_on = ? AND deleted_at IS NULL
```

Indexed, fast. Returns all tasks that are directly waiting on the given key.

## Cycle Detection

Setting `depends_on` must not create a cycle. Walk the chain from the
target task. If the source task appears anywhere in the chain, reject.

```go
func (s *Store) setDep(key, depKey string) error {
    if key == depKey {
        return sdk.ErrCycle
    }

    // Walk depKey's chain. If we encounter key, it's a cycle.
    current := depKey
    seen := map[string]bool{key: true}
    for current != "" {
        if seen[current] {
            return sdk.ErrCycle
        }
        seen[current] = true
        t, err := s.Read(current)
        if err != nil {
            return err
        }
        current = t.DependsOn
    }

    // Safe to set
    ...
}
```

Error: `sdk.ErrCycle` (new sentinel) → maps to HTTP 409 Conflict.

## Events

Dependency changes emit `task.updated` with metadata:

- `{"field": "depends_on", "old": "", "new": "b7e2j4n9p"}` - dependency set
- `{"field": "depends_on", "old": "b7e2j4n9p", "new": ""}` - dependency cleared

These flow through the bus → queue → SSE/webhooks as normal.

## Audit Log

Recorded in task history:

- Action: `"edited:depends_on"`, old_value: previous key (or ""), new_value: new key (or "")

Consistent with existing `"edited:title"`, `"edited:assigned_to"`, etc.

## Edge Cases

- **Deleted dependency:** if the depended-on task is soft-deleted, treat as
  unmet. The task cannot proceed until the dependency is restored and
  completed, or the dependency is cleared.
- **Self-dependency:** reject with `ErrCycle`.
- **Setting same dependency again:** idempotent, no-op.
- **Clearing non-existent dependency:** idempotent, no-op.
- **Non-existent dep key:** reject with `ErrNotFound`.
- **Chain depth:** no hard limit. The Go loop walks until it hits a task
  with no dependency. Practically, chains longer than ~10 would be unusual.

## Orchestrator Integration

The orchestrator uses `Ready(key)` before spawning:

```go
for _, task := range todoTasks {
    ok, _ := ctx.Tasks.Ready(task.Key)
    if ok {
        spawn(ctx, task)
    }
}
```

On task completion, check who's now unblocked:

```go
dependents, _ := ctx.Tasks.Dependents(completedKey)
for _, dt := range dependents {
    ok, _ := ctx.Tasks.Ready(dt.Key)
    if ok && dt.Status == "todo" {
        spawn(ctx, dt)
    }
}
```

## Implementation Order

1. Add `depends_on` column to tasks table
2. Update `Task` model to include `DependsOn string`
3. Update `TaskAddOpts` and `TaskSetOpts` to support `--depends-on`
4. Add cycle detection to set operation
5. Add `Dep`, `Dependents`, `Chain`, `Ready` methods
6. Add `task chain` and `task ready` CLI commands
7. Update `task list` to show dependency column
8. Emit events and audit log entries for dependency changes
9. Add smoke tests
