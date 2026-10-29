# Architecture Reference

## Running llmd

If `llmd` is not on your PATH, use the binary built in the project root
(`./llmd`). Try the PATH first; fall back to `./llmd` if not found.

## Core Principle

**The SDK is the single API surface. Everything goes through it.** CLI,
MCP, HTTP (coming), extensions - all are thin consumers that call SDK
interfaces. No domain logic lives in consumer layers.

This is deliberate: the API controls all actions and interactions through
a consistent code contract. Changes to internals are hidden behind the
SDK boundary and don't impact consumers. If something is available to
one consumer, it must be available to all of them through the SDK.

Every domain follows the same pattern:
1. Define the interface in `sdk/`
2. Implement in `internal/`
3. Bridge in `internal/host/`
4. Consumer layers (CLI, MCP, HTTP) are thin callers - no domain logic

Use structs-as-options for method parameters so that internal changes
stay hidden behind the SDK boundary.

**If logic is in a consumer layer and could be needed by another consumer,
it's in the wrong place.** Move it behind the SDK.

## Package Map

```
sdk/                        API surface: interfaces, types, flag parsing, sentinel errors
app/                        Build-time metadata: version tag, commit, build time
cli/                        Thin CLI dispatch: calls SDK, formats output
extension/                  Caddy-style compile-time plugin registry
internal/host/              SDK bridge: translates SDK calls to internal packages
internal/llmd/              Store: opens databases, coordinates sub-packages
internal/llmd/documents/    Document CRUD, versioning, soft-delete
internal/llmd/history/      Version listings, unified diffs, reverts
internal/llmd/search/       FTS5 full-text search and glob matching
internal/llmd/fts/          Keeps the FTS5 index in step inside each document transaction
internal/llmd/tasks/        Task board: columns, metadata, audit log
internal/llmd/tags/         Key-value tags attached to documents
internal/llmd/links/        Directed relationships between documents
internal/llmd/bulk/         Import from / export to filesystem (os.OpenRoot confined)
internal/llmd/events/       Internal synchronous event bus
internal/llmd/entities/     Named entity extraction
internal/llmd/audits/       Agent-to-agent and human-to-agent review threads
internal/llmd/audit/        Change log
internal/llmd/key/          ID generation: 10-char base36 from ms timestamps
internal/llmd/lazy/         One-time setup that retries after a failure (unlike sync.Once)
internal/llmd/hash/         Content hashing (xxh3)
internal/llmd/meta/         Document metadata helpers
internal/git/               Git CLI wrapper: sdk.GitStore implementation (build-tagged)
internal/config/            Configuration files and .llmd/.gitignore management
internal/line/              Platform-aware line ending conversion (build-tagged)
internal/validate/          Input validation: null bytes, path length, content size
internal/server/            HTTP API server: thin transport over sdk.Runtime.Dispatch
internal/term/              Terminal detection and stdin reading utilities
internal/sql/               Schema for llmd.db
pkg/model/core/             Core model types (Origin)
pkg/events/                 Shared event type definitions
guide/                      User-facing command guides (markdown)
```

## Command Flow

The CLI entry point is split across focused files in the `main` package:

- `main.go` - orchestration only (~114 lines)
- `flags.go` - `parseGlobal()` extracts global flags from args
- `output.go` - `display()` renders sdk.Response, `errorf()` formats errors
- `log.go` - `initLog()` configures slog
- `help.go` - `printHelp()`, `printCmdHelp()`

Terminal detection and stdin reading live in `internal/term/`.

```
main.go → run()
  ↓ parseGlobal(args) - flags.go
  ↓ config.Load() + initLog() - log.go
  ↓ signal.NotifyContext - Ctrl+C cancellation
  ↓ check extension.Storeless - skip store for init, version, config
  ↓ host.Open(dbPath) or host.New() depending on needsStore
  │   ├─ load compiled extensions via extension.All()
  │   └─ wire extension EventHandlers to internal bus
  ↓ resolve author (flag → config → term.Interactive() fallback)
  ↓ term.Stdin() - internal/term/ (a source; nothing is read yet)
  ↓ host.Exec(ctx, cmd, args, author, stdin, dbPath)
  │   └─ host.Context(ctx, author): per-request bridges bound to ctx,
  │      plus the host itself as sdk.Runtime
  ↓ extension.Exec(sctx, cmd, args) → sdk.Response
  ↓ display(result, jsonOut) - output.go
```

### Input

`sdk.Context.Stdin` is an input source, not bytes: the CLI's stdin
when it is a pipe or file, the request content for MCP and HTTP, or
nil. Nothing reads it until a command asks:

- `ctx.Input()` - required input (`write`, `hook`). Reads to the end
  however slow the writer, and stops on Ctrl-C.
- `ctx.OptionalInput()` - input that is one of several sources
  (`task add`, `audit add|reply`, `queue send`). Waits 50 ms for the
  first byte, then reads to the end with no limit.

Agent harnesses often run commands with stdin as a pipe they never
close, so only read it when the command needs it: after positional
content and `--file` have been checked.

### Global flags

`parseGlobal` (`flags.go`) reads `--author`, `--db`, `--json`,
`--verbose` and `--help`/`-h` before or after the command, as
`--name value` or `--name=value`, the same in both places. After the
command, `--` ends global flags. Before the command an unknown flag is
an error. `--version`/`-v` are global only before the command (`llmd
--version`); after it they are the command's own (`cat --version 3`).
Version info falls back to `debug.ReadBuildInfo` when ldflags are not
set, so `go install` builds report their version and commit.

## Flag Parsing

All CLI commands parse flags through `sdk.ParseArgs`, which is driven by the
`sdk.Flag` metadata already defined on each `sdk.Command`. This gives a single
source of truth for `--help` display, MCP tool schemas, and argument parsing.

```go
flags, positional, err := sdk.ParseArgs(cmdSpec.Flags, args)
if err != nil {
    return nil, fmt.Errorf("cmd: %w", err)
}
version := flags.Int("version")
verbose := flags.Bool("n")
```

### Types (`sdk/flags.go`)

- `Flag` - describes one flag: `Name`, `Short` (single-char alias), `Type`
  (`"bool"`, `"string"`, `"int"`), `Desc`.
- `FlagValues` - returned by `ParseArgs`. Accessors: `Bool(name)`, `String(name)`,
  `Int(name)`, `Has(name)`. `Has` distinguishes "not provided" from "provided
  with zero value" (e.g. `--priority 0` vs omitted).
- `ParseArgs(flags []Flag, args []string) (FlagValues, []string, error)` - parses
  POSIX-style flags. Returns flag values and remaining positional arguments.

### Supported syntax

- `--name` (bool), `--name value`, `--name=value` (string/int)
- `-n` (short bool), `-lat` (combined short bools)
- `-C3` (compact short int), `-C 3` (separate short int)
- `-nC3` (combined: `n` is bool, `C` consumes `3`)
- `--` terminates flag parsing; everything after is positional

### Author enforcement on mixed commands

Commands that only change the store (write, edit, rm, mv, restore, revert,
sed, unlink, import, vacuum) declare `NeedsAuthor: true` on the
`sdk.Command`. The host rejects these before dispatch if no author is set.
The same flag makes them POST routes over HTTP, so no change to the store
can be made with a GET request.

Mixed commands (tag, link, task) support both reads and writes. They do NOT
set `NeedsAuthor` - instead, their handlers check `ctx.Author == ""` on
mutation paths only. This allows read operations (e.g. `tag -f`, `link <path>`,
`task list`) to work without an author.

Config author resolution only applies to interactive terminals. Non-interactive
callers (LLMs, scripts, MCP) must always provide `--author` explicitly.

## Domain Interfaces

Focused interfaces per domain, each following the SDK-first pattern:

| Interface | Defined in | Implemented by | Wired in |
|-----------|-----------|----------------|----------|
| `sdk.DocumentStore` | `sdk/documents.go` | `internal/host/api_documents.go` (`documentAPI`) | `internal/host/host.go` |
| `sdk.TaskStore` | `sdk/tasks.go` | `internal/host/api_tasks.go` (`taskAPI`) | `internal/host/host.go` |
| `sdk.LinkStore` | `sdk/links.go` | `internal/host/api_links.go` (`linkAPI`) | `internal/host/host.go` |
| `sdk.TagStore` | `sdk/tags.go` | `internal/host/api_tags.go` (`tagAPI`) | `internal/host/host.go` |
| `sdk.AuditStore` | `sdk/audits.go` | `internal/host/api_audits.go` (`auditAPI`) | `internal/host/host.go` |
| `sdk.ActivityStore` | `sdk/activity.go` | `internal/host/api_activity.go` (`activityAPI`) | `internal/host/host.go` |
| `sdk.HookStore` | `sdk/hooks.go` | `internal/host/api_hooks.go` (`hookAPI`) | `internal/host/host.go` |
| `sdk.GitStore` | `sdk/git.go` | `internal/git/git.go` (`Git`) | `internal/host/host.go` |

Each bridge type in `internal/host/` holds a `context.Context` field and
translates SDK flat arguments into internal option structs, passing the
bound context to every internal store call. All mutating operations stamp
a `core.Origin{Source: "cli"}` for audit tracking.

**Context-Bound Bridges:** `host.Context(ctx, author)` builds the
`sdk.Context` a command receives: fresh bridge instances bound to the
request's `context.Context`, so cancellation and timeouts propagate
without changing any SDK interface signatures. `sdk.Context` embeds
`context.Context` and carries the domain store fields (`Documents`,
`Tasks`, `Links`, `Tags`, `Activities`, `Audits`, `Queue`, `Mirror`,
`Agents`, `Rules`, `Git`) plus `Runtime`. Commands access everything
through the context; **the `sdk` package has no mutable package-level
state**.

`sdk.Runtime` is the host's own surface, implemented by `host.Host` and
carried on every context. It is for commands that reach back into the
host rather than into a domain: `Dispatch` (the MCP and HTTP servers),
`Commands` (tool and route registration), `Subscribe` (SSE and
webhooks), `Watch` (changes made by other processes, for the server)
and `Init` (the `init` command). Tests get the same context a
command would via `host.TestSetup(t, mode).Context(ctx, author)`.

`sdk.GitStore` has no host bridge - `internal/git.Git` implements it
directly. It is a system utility independent of the store, so a bridge
would add indirection without value. The host owns one instance and
passes it to the task and agent bridges and onto the context.

Bridge types also translate internal errors to SDK sentinels via per-domain
helpers (`docErr`, `taskErr`, `linkErr`, `tagErr`). This prevents internal
error types from leaking through the SDK boundary:

| Internal error | SDK sentinel |
|---------------|-------------|
| `documents.ErrNotFound` | `sdk.ErrNotFound` |
| `documents.ErrDeleted` | `sdk.ErrNotFound` |
| `documents.ErrExists`, `documents.ErrNotDeleted` | `sdk.ErrExists` |
| `history.ErrNotFound` | `sdk.ErrNotFound` |
| `tasks.ErrNotFound` | `sdk.ErrNotFound` |
| `tasks.ErrNoSpec` | `sdk.ErrNoSpec` |
| `tasks.ErrMissingTitle` | `sdk.ErrMissingArg` |
| `tasks.ErrInvalidCol` | `sdk.ErrInvalidArg` |
| `tasks.ErrColNotFound` | `sdk.ErrNotFound` |
| `tasks.ErrColExists` | `sdk.ErrExists` |
| `tasks.ErrColNotEmpty` | `sdk.ErrInvalidArg` |
| `links.ErrNotFound` | `sdk.ErrNotFound` |
| `links.ErrSelfLink` | `sdk.ErrInvalidArg` |
| `links.ErrExists` | `sdk.ErrExists` |
| `tags.ErrNotFound` | `sdk.ErrNotFound` |
| `tags.ErrInvalid` | `sdk.ErrInvalidArg` |
| `tags.ErrExists` | `sdk.ErrExists` |
| `audits.ErrNotFound` | `sdk.ErrNotFound` |
| `audits.ErrMissingAuthor` | `sdk.ErrMissingArg` |
| `audits.ErrMissingTarget` | `sdk.ErrMissingArg` |
| `history.ErrVersionInvalid` | `sdk.ErrInvalidArg` |
| `documents.ErrNotFound` (tag or link target) | `sdk.ErrNotFound` |
| `agents.ErrNotFound`, `agents.ErrRunNotFound` | `sdk.ErrNotFound` |
| `agents.ErrRunning` | `sdk.ErrAgentRunning` |
| `agents.ErrNotRunning` | `sdk.ErrAgentNotRunning` |
| `messages.ErrNotFound` | `sdk.ErrNotFound` |
| `messages.ErrOrderViolation` | `sdk.ErrOrderViolation` |

CLI commands use context-local stores:

```go
ctx.Documents.Read("path", 0)
ctx.Tasks.Add("title", body, sdk.TaskAddOpts{Author: ctx.Author})
ctx.Tags.Add("path", "name", ctx.Author)
ctx.Links.Add("a", "b", "label", ctx.Author)
```

## Extension System

Extensions are registered at init-time via `extension.Register()`, following
the `database/sql.Register` pattern. The `cli` package uses this. Panics on
duplicate names to catch programmer errors early. Extensions implement
`sdk.Plugin` (Name, Commands, Exec) and receive raw args, parsing flags
themselves.

## Event System

Three separate event mechanisms serve different layers:

1. **Internal bus** (`internal/llmd/events/`) - synchronous, in-process,
   fired after the change commits. Subscribers are the message queue,
   extensions, the agent pipeline and the server's SSE and webhook
   clients. A failing handler is logged and the rest still run; `Emit`
   returns nothing, because the change it reports has already
   committed. Anything that must never miss a change belongs in the
   transaction instead: the FTS5 index is not a subscriber, it is
   updated by `internal/llmd/fts` inside each document transaction.
   Uses `pkg/events.Event` as the event type.

2. **Shared event types** (`pkg/events/`) - struct definitions for
   `document.written`, `document.deleted`, `document.restored`,
   `document.moved`. Consumed by the internal bus.

   The bus is in-process: it sees only this process's mutations. For
   mutations by other processes, `sdk.Runtime.Watch` polls SQLite's
   `data_version` on each database (`internal/llmd/watch.go`) and hands
   the host's Subscribe callbacks a `store.changed` event. That event
   deliberately bypasses the bus, so the search index, message queue
   and extensions never see it; `llmd serve` starts the watcher. Our
   own bus events arrive only after the commit, so a change is held one
   interval and dropped if an event of ours arrived meanwhile.

3. **Extension events** (`extension/events.go`) - fire-and-forget notifications
   for extensions. Eight event types: document write/delete/restore/move, tag
   add/remove, link create/remove. Extensions implement `EventHandler` to
   observe. The host bridges internal bus events to extension handlers via
   `internal/host/events.go`. Extensions cannot veto operations.

## Key Generation

`internal/llmd/key/` generates 10-character base36 identifiers from millisecond
timestamps plus an atomic counter. These are NOT nanoid. Format: lowercase
alphanumeric, lexicographically sortable by creation time. Used for all entity
IDs (documents, tasks, audits). **IDs are bare keys with no prefix**  - 
never add `aud_`, `task_`, `doc_`, or any other prefix.

The counter keeps keys unique within one process. Separate processes seed
it at random, so two llmd processes started within the same second clash
about once per thousand keys. Every insert therefore goes through
`key.Insert`, which asks for a fresh key when the store refuses one as
taken; `tasks.key`, `entities.key`, `agent_runs.key`, `messages.key` and
`audits.id` are unique in the schema, and a new document checks its key
against `content` inside the write transaction (versions of one path
share a key, so that table cannot be unique on it). `key.Force` lets a
test make a clash happen.

## Testing

```bash
go test ./...
```

**Tests run against real stores on disk, never in-memory SQLite.** An
in-memory database skips WAL files, checkpoints and the on-demand
work.db, which are part of the behaviour under test. Three helpers cover
every layer:

- `host.TestSetup(t, mode)` initialises a store, returns a `*host.Host`
  over it and registers cleanup. `h.Context(t.Context(), author)` gives
  the `sdk.Context` a command would receive; `h.Dir()` is the project
  directory. `host.TestIsolated` creates the store in a temp directory
  and leaves the working directory alone; `host.TestWorkdir` also changes
  into that directory for cwd-relative behaviour (git, `.llmd/config`,
  `.llmd/.gitignore`). Host tests use `TestIsolated`; CLI git tests use
  `TestWorkdir`.
- `llmdtest.Open(t)` (`internal/llmd/llmdtest`) returns an `*llmd.Store`
  initialised under a temp directory, for package tests that go through
  the store.
- `sqlitetest.Open(t, name)` (`internal/sqlite/sqlitetest`) returns a
  bare `*qwr.Manager` on a temp file with production settings, for
  packages that are constructed directly from a database (tasks,
  audits, messages, agents, sql).

Database settings live in one place, `internal/sqlite.Open`, used by
the store for llmd.db and work.db and by the test helpers.

**Table-driven by default** - tests that exercise one operation with
several inputs or expectations are written as a single `TestX` with a
`tests := []struct{ name string; ... }` table and `t.Run(tt.name, ...)`
subtests. See `sdk/flags_test.go`, `internal/path/path_test.go` and
`internal/llmd/key/key_test.go` for the idiom. Conventions:

- Declarative `want` / `wantErr` fields over check callbacks; compare
  errors with `errors.Is` against the package sentinels.
- A `setup func(t *testing.T, s *llmd.Store)` field when cases need
  different precondition state.
- Every subtest opens its own store (`testStore(t)` or
  `host.TestSetup`). `host.TestWorkdir` and the git tests change the
  process working directory, so those must not use `t.Parallel()`.
- Failure messages read `Func(args) = got, want %v`.
- Use `t.Context()` rather than `context.Background()`.

Genuinely sequential scenarios (multi-step workflows such as
write/edit/revert, git start/branch/finish flows, the host integration
suite) stay as plain tests and may use `t.Run` for phases.

**Toolchain** - the minimum Go version is 1.27.0 (`go.mod`). Run
`make tidy` before committing; it includes `go fix ./...`, whose
modernisers are applied as-is (for example promoted fields in composite
literals, `min`/`max`, `for range n`).

## CLI Views

Three static terminal views provide human-friendly overviews. All use lipgloss
v2 (`charm.land/lipgloss/v2`) for styled output in terminals and fall back to
plain text when piped (see "Two Audiences, One Source").

| Command | File | Description |
|---------|------|-------------|
| `diff` | `cli/diff.go` | Coloured unified diffs (green/red/cyan) when TTY |
| `status` | `cli/status.go` | Dashboard: recent docs, task summary, activity |
| `review` | `cli/review.go` | Pending tasks with spec previews and links |
| `ls --tree` | `cli/ls.go` | Directory hierarchy via `lipgloss/v2/tree` |

Shared styles live in `cli/styles.go`: table styles (`tblHeader`, `tblCell`,
`tblDim`, `tblBorder`) and diff colour styles (`diffAdded`, `diffRemoved`,
`diffHunk`, `diffHeader`). View-specific styles are local to each command file.

The `status` command uses a unified activity feed (`ctx.Activities.Recent()`)
that queries documents, entities (tags/links), and task audit events in parallel,
then merges by timestamp. The feed is defined as `sdk.ActivityStore` (in
`sdk/activity.go`), implemented in `internal/llmd/activity.go`, bridged through
`internal/host/api_activity.go` (`activityAPI`), and wired in `internal/host/host.go`.

Task audit actions use past tense: `"created"`, `"moved"`, `"deleted"`,
`"restored"`, `"edited:*"`, `"flagged"`, `"unflagged"`.

## Git-Aware Tasks

Tasks can be linked to git branches via the `Branch` field. Multi-step
orchestration (start, finish, branch creation) lives in the SDK so all
consumers (CLI, MCP, HTTP) can use it. The CLI provides thin
dispatch on top.

### SDK methods (`sdk.TaskStore`)

| Method | Description |
|--------|-------------|
| `Start(key, author, StartOpts)` | Move to column + record current git branch (git optional) |
| `StartBranch(key, author, StartBranchOpts)` | Create branch from title slug, checkout, record, move to column |
| `Finish(key, author, FinishOpts)` | Move to done + return `FinishResult` with git summary counts |
| `Next(author, NextOpts)` | Claim the first ready task of a column: move + assign in one transaction (`internal/llmd/tasks/claim.go`) |
| `ByBranch(branch)` | Find task linked to a branch name |

Within a column, `List` and `Next` share one order (`internal/llmd/tasks/order.go`):
by default priority first (1 is the highest, 0 is none, negatives are
refused), then position; `task.order: position` in the config makes
position the whole order. The host reads the key in `setup` and applies
it on `attach` with `Tasks.SetOrder`; tests pass `host.WithTaskOrder`.

Implementation lives in `internal/host/api_tasks.go`. The `branchSlug`
helper (title → git-safe branch name) is unexported in the same file.

### CLI subcommands

| Command | File | Description |
|---------|------|-------------|
| `task start <id>` | `cli/task_git.go` | Parse flags → `ctx.Tasks.Start()` → format text |
| `task finish [id]` | `cli/task_git.go` | Parse flags → `ctx.Tasks.Finish()` → format summary |
| `task branch <id>` | `cli/task_git.go` | Parse flags → `ctx.Tasks.StartBranch()` → format text |
| `task diff [id]` | `cli/task_git.go` | Resolve task + `ctx.Git.Diff()` → coloured output |
| `task files [id]` | `cli/task_git.go` | Resolve task + `ctx.Git.Files()` → file list |
| `task commits [id]` | `cli/task_git.go` | Resolve task + `ctx.Git.Commits()` → commit list |

Commands marked `[id]` auto-detect the task from the current git branch
when no ID is given (`ctx.Tasks.ByBranch` via the current branch).
`task show` displays ahead/behind counts when the task has a branch
and git is available.

All git operations degrade gracefully - `Start` and `Finish` work
without git (skipping branch recording and git summary respectively);
`StartBranch` requires git and returns a clear error if unavailable.
Default branch detection tries `origin/HEAD` first, then `main`, then
`master`; override with `--base`.

## Audit Threads

The audit domain (`sdk.AuditStore`) provides agent-to-agent and human-to-agent
review threads. Audits are **insert-only** - no record is ever updated. Thread
status is derived from the most recent entry.

**Data model:** A single `audits` table. Top-level audits target a document
path or task key. Replies reference a `parent_id`. The store resolves replies
to the top-level ancestor (no nested threads). `deleted_at` is the one
pragmatic mutation - a visibility flag for soft-delete.

**Author vs assignee:** `author` is who created the entry. `assignee` is who
needs to act on it. These are separate fields - an agent creates an audit
(`--author`) and directs it to another agent (`--assign`). The assignee
propagates through replies like status does; the effective assignee is from
the most recent entry. `audit status` filters by effective assignee.

**ID format:** Bare 10-char base36 key with no prefix (same generator as tasks
and all other entities). IDs are never prefixed - no `aud_`, `task_`, `doc_`.

**Target type inference:** The store determines `target_type` from the target
value. If it matches a valid 10-char base36 key, it's a task; otherwise it's
a document.

**Effective status:** Thread status = status of the most recent non-deleted
entry. Queries use `ORDER BY created_at DESC, id DESC LIMIT 1` (the `id DESC`
tiebreaker handles same-millisecond key generation).

### CLI subcommands

| Command | Description |
|---------|-------------|
| `audit add <target> [content]` | Create a top-level audit |
| `audit reply <id> [content]` | Reply to an existing thread |
| `audit list [target]` | List audits (filterable by `--assign`, `--by-author`, `--status`, `--pending`) |
| `audit show <id>` | Display full audit thread |
| `audit resolve <id>` | Mark as approved (inserts "approved" entry) |
| `audit rm <id>` | Soft-delete |
| `audit restore <id>` | Recover a soft-deleted audit |
| `audit status` | Inbox: pending threads assigned to the author |

Content resolution order: positional args > `--file` > stdin.

The `audit` command is a mixed command (like `task`): author is required for
mutations (`add`, `reply`, `resolve`, `rm`, `status`) but not for reads
(`list`, `show`).

## Agent Hooks

Agent platforms run `llmd --author <name> hook` at points in a session
and pass their event payload on stdin. `sdk.HookStore.Handle` is the
only place that decides what an event does: the author selects the
platform parser (`assets/platform`), the payload becomes a
`platform.HookEvent`, and the host acts through the queue, task and
audit bridges. The `hook` command, `POST /hook` and the MCP tool are
thin wrappers over it. `hook init <platform>` prints configuration that
runs that command; spawned agents get it in the settings written to
their worktree. Do not add hook behaviour to a transport.

## Message Queue

The message queue (`llmd queue`) is a persistent, ordered notification layer.
Domain events and direct messages land in the same queue. Consumers poll,
process front to back, and acknowledge each message.

**Storage:** Two insert-only tables (`messages`, `message_acks`), created
lazily on first use. Messages carry a `source_key` for cross-process
deduplication and an `assigned_to` for directed messages.

**SDK interface:** `QueueStore` in `sdk/queue.go` with `Send`, `Pending`,
`Peek`, `Ack`, `History`. Implementation in `internal/llmd/messages/`.
Host bridge in `internal/host/api_queue.go`.

**Event bus wiring:** `messages.QueueHandler` subscribes to the internal
bus and publishes domain events as queue messages. Skips `message.*` events
to avoid feedback loops. The handler only queues when work.db is open -
document-only stores skip queue writes silently. Wired in `Store.wire()`.

**Strict ordering:** `Ack` rejects if the key does not match the consumer's
oldest pending message (`ErrOrderViolation`).

**Retention:** nothing removes a message except `Store.Vacuum`, which calls
`Messages.Purge` for messages older than the store's queue retention
(`queue.retention` in the config, default 30d; 0 keeps everything). The
host reads the key in `setup` and applies it on `attach` with
`SetQueueRetention`; tests pass `host.WithQueueRetention`. Age is the only
test - a consumer that never acknowledges does not pin the table.

**Directed vs broadcast:** Messages with `assigned_to` set are only visible
to that consumer. NULL means broadcast - all consumers see it.

### CLI subcommands

| Command | Description |
|---------|-------------|
| `queue send <content> [--assign X]` | Send a message |
| `queue ls [--limit N]` | Pending messages for current author |
| `queue peek` | Next unacknowledged message |
| `queue ack <key>` | Acknowledge oldest pending |
| `queue history [--since]` | All messages including acknowledged |

All subcommands require `--author`.

## Transaction Patterns

All database access goes through `qwr.Manager`, which provides serialised
writes with reader/writer connection separation. Multi-statement write
operations use `qwr.TransactionFunc` - a callback-based pattern that
handles `Begin`/`Commit`/`Rollback` automatically:

```go
result, err := db.TransactionFunc(func(tx *sql.Tx) (any, error) {
    // all writes here are atomic
    return value, nil
}).WithContext(ctx).Write()
```

The callback receives a raw `*sql.Tx`. The return value is accessible via
`result.Value` with a type assertion.

**Documents:** `Write` delegates to `writeInTx` inside a `TransactionFunc`.
The event bus fires *after* commit so subscribers see committed data.
`writeInTx` and `readInTx` exist for callers that need document operations
within a larger transaction.

**Tasks:** `Add`, `Move`, `Set`, `Delete`, and `Restore` each use
`TransactionFunc`, perform their UPDATE/INSERT, write the audit record via
`recordTx`, and commit. `recordTx` (in `helpers.go`) mirrors `audit.Log.Record`
but writes on the provided `*sql.Tx`. `repositionTx` (in `move.go`) renumbers
column positions within a transaction.

`Tasks.ensure()` calls `audit.Log.Ensure()` to guarantee the history table
exists before any `recordTx` call. `audit.Log.Ensure` is exported and
idempotent (guarded by `sync.Once`).

**Links:** `Remove` wraps its soft-delete loop in a `TransactionFunc` so
multiple link deletions are atomic.

**History revert:** `Revert` reads the source version content outside the
transaction, then uses `TransactionFunc` for the atomic
check-latest-hash → increment-version → insert sequence. This prevents
concurrent reverts from producing duplicate version numbers.

**Document states:** a document is live, deleted or absent as a whole:
every version of a path shares one state, and nothing leaves a path
part live, part deleted. `documents.Check` (and `CheckTx` inside a
transaction) is the one place that reads the state: nil, `ErrDeleted`
or `ErrNotFound`. Every operation applies the same rule:

| Operation | Live | Deleted | Absent |
|-----------|------|---------|--------|
| read, stat, history, diff, edit, sed, export, revert, rm, mv (source), cp (source), tag, link, task `--path` | proceeds | `ErrDeleted` | `ErrNotFound` |
| write, cp (destination), import | new version | continues the document: earlier versions return, the write is the next version | version 1 |
| restore | `ErrNotDeleted` | restores every version | `ErrNotFound` |
| mv (destination) | `ErrExists` | `ErrExists` (restore or vacuum it) | proceeds |

Keys resolve to their document in any state (`KeyToPath`), so a key
reaches the same result as its path. A deleted document still owns its
path: `task add` picks `tasks/<slug>-2` rather than continuing one.
`vacuum` purges only documents deleted as a whole. Changing a rule
means changing `TestDocumentStates` (`internal/llmd/lifecycle_test.go`).

**Search index:** `content_fts` is an external-content FTS5 table, so
removing a row means replaying the exact values it was indexed with,
and replaying a row that is not indexed corrupts the index. The index
holds one row per path with a live version: its latest live version.
Every change to `content` (`writeInTx`, `Delete`, `Restore`, `Move`,
`revertInTx`) calls `fts.Live` before the change and `fts.Sync` after
it on the same transaction, so the index commits or rolls back with
the change. A write that changes nothing adds no version and emits no
event.

**Work database:** `Store.WorkDB` opens work.db under a mutex, and each
work domain guards its setup with `lazy.Once`, so concurrent server
requests open it once and a failed open is retried on the next call.
`Agents.Record` checks for a running agent and inserts the run in one
transaction, so two spawns for one task cannot both start.

**When to use `recordTx` vs `audit.Record`:** Use `recordTx` inside a
transaction to make audit records atomic with the surrounding operation.
Use `audit.Record` only for standalone operations that do not share a
transaction.

## Two Audiences, One Source

llmd is used by people at a terminal and by agents, through the same
commands. The facts are the same for both; the shape differs. A person
wants prose, colour and tables. A model pays for every word, cannot
scan a table, and acts on error text. llmd serves both from one source
rather than two command sets or two sets of documentation, which would
drift apart.

**Command help.** The fields on `sdk.Command` (`Desc`, `Usage`,
`Flags`, and optionally `Brief`) are written once. Three renderings
read them:

| Rendering | Where | Built by |
|-----------|-------|----------|
| Terminal `--help` | `llmd <cmd> --help` at a TTY | `cmdHelp(c, true)` in `help.go`: whole `Desc`, usage, flags, global flags |
| Agent `--help` | `llmd <cmd> --help` piped | `cmdHelp(c, false)`: `c.AgentHelp()` plus one line of global flags |
| MCP tool list | every MCP session start | `registerTool` in `cli/mcp.go`: `c.AgentHelp()` |

`Command.Guide` names the guide page a command's help points to
("Details: guide <page>"): empty for the command's own name, `-` for
none (`version`, `extensions`). `TestGuidePointers` checks every
pointer resolves, and `TestHelp` that root help lists every command
and global flag.

`Command.Summary()` is the first paragraph of `Desc`. `AgentHelp()` is
`Summary` plus the flags and `Details: guide <name>`, or `Brief` alone
when set. Set `Brief` on a command whose generated form would run long
(subcommands, many flags) and keep it to a few lines; the MCP tool
list is held under 14 KB by `TestMCPToolListBudget`. `mcpInstructions`
in `cli/mcp.go` is the one message a model sees at connect time; the
`guide` index page says the same things for a model in a shell.

**Command output.** `sdk.Context.TTY` chooses the shape, not just the
colour. The host sets it for the CLI when stdout is a terminal and
never for MCP or HTTP: a server started in a terminal still answers
its clients in plain form. Commands read `ctx.TTY`, never the process's
stdout. Piped output is for a model or a script:

- `newTable` returns a `grid` (`cli/styles.go`): a bordered lipgloss
  table at a terminal, tab-separated lines with a header line when
  piped. Every tabular command uses it; never call `table.New`
  directly for a list.
- Tasks print as `plainTaskLine` (`cli/task_board.go`): seven
  tab-separated fields, `key title column P<n> assignee flags spec`,
  shared by `task list` and `review`. `task list` groups them under
  `COLUMN (count)` headings.
- Markdown (`cat`, `guide`, `task show`) is rendered at a terminal and
  raw when piped (`renderMarkdown` in `output.go`), in the
  `display.theme` style (`dark` or `light`). The theme is configured,
  not detected: asking the terminal costs seconds when it does not
  answer. Table headers and headings are bold in the terminal's own
  foreground colour, never a fixed colour, so they read on any
  background.
- Output that grows without bound is capped and says so, with `--all`
  to lift the cap (`ls`, `find` and `glob` say it in `sdk.Result.Note`,
  which the CLI prints on stderr so piped lists stay one item per line,
  and MCP appends to the text): `ls` 500 documents, `diff` 500 lines, `task list`
  the newest 10 done tasks, `review` leaves out done and human tasks.
- Error text names the next command to run where there is one (spec
  gating, `task next` with nothing ready).

**JSON output.** `--json` (and `Output: json` over HTTP) means JSON
from every command, decided in one place: `sdk.JSON(resp)` is the
response's data when it has some, otherwise `{"text": "<its text>"}`.
An object, not a bare string, so a command can add fields later without
breaking a parser. Keys are snake_case: every SDK type carried as data
has `json` tags. `TestJSONEverywhere` (`json_test.go`) runs commands
with `--json` and fails on output that is not JSON or a key with a
capital letter. MCP stays a text surface: its tools return the text
form.

**Guides.** Every page in `guide/` is dense-first: a title, one plain
sentence, `## Usage`, `## Flags`, `## Rules` (what is required, refused,
capped, and what piped output looks like), `## Examples`, then prose.
A model that reads only the first screen still makes the right call;
a person reads on. `TestGuideShape` and `TestGuideFlagsExist` in
`cli/guide_test.go` hold pages to the shape and to the command specs:
every flag in a spec appears on its page, and every flag a page uses
for its command exists. Per-subcommand flags are listed by hand in
`subcommandFlags` there; add to it when a subcommand gains a flag.

## MCP Server

The MCP server (`cli/mcp.go`) exposes llmd commands as MCP tools over stdio.
All tools share a single `toolInput` schema with `args`, `content`, and `author`
fields. The `author` field lets the LLM identify itself per-call - this is the
primary author source in MCP mode, since there is no interactive user.

If the LLM omits `author` and the command has `NeedsAuthor: true`, the server
rejects the call immediately. Mixed commands (tag, link, task) do not set
`NeedsAuthor` - their handlers check author on mutation paths only, so read
operations succeed without an author.

## HTTP Server

The HTTP server (`internal/server/`) exposes llmd commands as HTTP endpoints.
It follows the same pattern as the MCP server - a thin transport layer over
`sdk.Runtime.Dispatch`. Commands are registered automatically by walking
`sdk.Runtime.Commands()`, so plugins get HTTP routes for free.

**Route structure:** URL paths mirror CLI commands:
`/<command>/<subcommand>/<arguments>`. `/cat/docs/api` dispatches to `cat`
with the path `docs/api`; `/task/move/abc/review` dispatches to `task` with
`move abc review`. `/grep?q=hello` passes the query as the first positional
argument. A document path is always the last part of the URL so it can hold
slashes; an earlier path is URL-encoded as one segment.

**Routes are declared:** a command with subcommands lists its routes in
`sdk.Command.Routes` (see `cli/routes.go`): method, pattern and the
argument template the URL expands into: `{name}` is a part of the URL,
`{?name}` a query parameter carrying free text (edit's old and new
text, a sed expression), after which the flags end with `--`. A
command without routes gets the
default pair `GET /<name>` and `GET /<name>/{path...}`, or `POST` when it
has `NeedsAuthor`. Reads are GET; changes are POST or DELETE. Tests in
`cli/routes_test.go` check that every GET route works without an author
(so none can mutate), that templates and patterns agree, and that
`guide/serve.md` lists every declared route.

**Headers carry metadata:** `Author` (fallback identity), `Message` (commit
message), `Output` (`json` to request structured
data). No `X-` prefix - RFC 6648 deprecated it.

**Request body is content:** POST bodies carry raw document content (markdown),
not JSON envelopes. This matches the CLI where stdin is the document body.

**Response format:** `sdk.Text` returns `text/plain`, `sdk.Markdown`
`text/markdown`, `sdk.Result` its text, and `sdk.Data` JSON. With the
`Output: json` header (or with no text to return) the body is
`sdk.JSON(resp)`, the same value the CLI prints for `--json` (see JSON
output).

**Error mapping:** `statusOf` (`internal/server/handler.go`) maps every
SDK error kind to a status, and `TestStatusOfEveryKind` fails when one
is missing: `ErrNotFound`, `ErrUnknownCmd` → 404; `ErrMissingArg`,
`ErrInvalidArg` → 400; `ErrExists`, `ErrOrderViolation`, `ErrCycle`,
`ErrAgentRunning`, `ErrAgentNotRunning`, `ErrNotReady`, `ErrNoneReady`,
`ErrUnavailable` (git missing or not a repository) → 409; `ErrNoSpec`,
`ErrNoMatch`, `ErrNotUnique`, `ErrNoOp` → 422. The router's own 404 and
405 replies are JSON too. All errors return JSON
`{"error": "..."}` with the message as the CLI prints it (see Errors).

**Configuration:** Listen address is read from the `server.addr`
config key, defaulting to `localhost:5563`. No flags or environment
variables.

**Hardening:** without `server.token` the server answers only requests
whose `Host` (and `Origin`, when sent) is loopback or the configured
listen host, so a web page cannot reach it through DNS rebinding, and
`ListenAndServe` refuses a non-loopback address. With a token, the
token is the check. Request bodies are capped (`WithMaxBody`, set from
`limits.content_size`), headers and requests time out
(`WithTimeouts`), and shutdown ends `/events` streams so it completes
instead of waiting out `WithShutdownTimeout`. URL values that start
with `-` are refused, since commands would read them as flags; query
flags are passed in name order. Refs and paths given to git are
refused when they start with `-`, revisions follow `--end-of-options`,
and each git command runs with a timeout and `GIT_TERMINAL_PROMPT=0`.

**Skipped commands:** `mcp`, `serve`, `init`, `config`, `version`,
`extensions` are not exposed over HTTP - they are admin or local-only.

The server uses `github.com/jpl-au/chain` as the HTTP mux, which wraps
Go 1.22's enhanced `net/http` routing with middleware support.

## Configuration

`internal/config` reads `.llmd/config.yaml`, or `~/.llmd/config.yaml`
when there is no local file. Every key has its default in
`config.Defaults()`, so `llmd config` shows what is in effect. YAML is
decoded leniently: unknown keys are ignored.

- `Config.Set` is the one place a value is checked; `llmd config <key>
  <value>` refuses what it refuses.
- `Config.Validate` runs `Set`'s checks over a loaded file (plus zero
  limits and webhook entries). `main` logs each problem once as a
  warning; nothing fails because of a bad value.
- Read durations and counts through `DurationSetting(key)` and
  `CountSetting(key, min)`, which return the default for a value
  `Validate` reports. Do not parse config strings at the use site.
- New tunable values go in the config with a default, not in a
  constant: a key in `Keys`, `Field` and `Set`, a default in
  `Defaults`, a row in `guide/config.md`, and an option on the
  component that uses it (`server.WithTimeouts`, `llmd.WithDatabase`,
  `git.New(timeout)`). Components keep working with their zero-value
  defaults so tests need no config.

## Gitignore Management

llmd manages `.llmd/.gitignore` using a whitelist approach: everything
is ignored by default, and specific files are allowed through with `!`
patterns. This means new files (telemetry logs, temp files, mirror
output) are automatically excluded without maintaining a growing
blocklist. llmd never touches the project's root `.gitignore`.

**Default `.llmd/.gitignore` (created by `llmd init`):**
```
*
!.gitignore
!llmd/
!llmd/**
```

Everything is ignored except the mirrored documents and the gitignore
itself. Documents are shared through git as the `.md` files `llmd
mirror` writes to `.llmd/llmd/`, which merge as text; `llmd.db` stays
local, since two people changing a binary database cannot be merged.
`work.db` (tasks, audits, agent activity) is local operational state.
`mirror push` deletes the documents whose files were removed, guided
by `.llmd/llmd.manifest` (local, never committed) so a document that
was never pulled is not mistaken for a removed one.

**CLI management:**
```bash
llmd config git ls                 # list current rules
llmd config git allow "reports/"   # allow reports/ to be committed
llmd config git deny "reports/"    # stop allowing reports/
```

## Logging

Uses `log/slog` from the standard library. The logger is initialised in
`main.go` before any host creation.

**Defaults:** level `warn`, format `text`, output to stderr. The CLI is
quiet unless something needs attention.

**`--verbose` flag:** overrides to `debug` level for instant diagnostics.

**Config keys** (`.llmd/config`, local or global):
- `log_level` - `debug`, `info`, `warn`, `error`
- `log_format` - `text`, `json`

`--json` implies `log_format=json` so structured output stays
machine-readable. `--verbose` overrides any configured level.

All packages use the process-wide `slog` default - call `slog.Debug`,
`slog.Info`, `slog.Warn`, `slog.Error` directly. No logger is passed
through context or structs.

## Errors

An error message is written for whoever reads it, a person or an
agent, and is the same from the CLI, MCP and HTTP:

- `<command>: <what is wrong>.` and, where there is one, a second
  sentence with the next step: `cat: p is deleted. Run llmd restore p
  to bring it back.`
- Name the thing once (the path, key or value), and say nothing about
  the code that found the problem: no `resolving x:`, no `getting
  version 9:`.
- No `x: y: z` chains and no ` - ` asides.

The error's *kind* is for code, not the message. Build errors with
`errs.New(kind, format, args...)` (internal packages) or
`sdk.Errorf(kind, ...)` (commands and extensions): `errors.Is(err,
kind)` holds and the transports map the kind to an exit code or HTTP
status, but the kind's own text is not added. The host classifies an
internal error as an SDK one with `errs.Mark(sdk.ErrX, err)`, which
adds a kind and no text. Never write `fmt.Errorf("%w: ...", ErrX)`.

- A sentinel with no context reads as a sentence on its own
  (`tasks.ErrNoneReady` is "no task is ready."), so a bare
  `fmt.Errorf("task next: %w", err)` still reads well.
- Errors that can name their object do, where they are returned:
  `documents.NotFound(path)`, `documents.Deleted(path)`,
  `documents.NoVersion(kind, path, v)`; tasks, audits, agents, tags
  and links build theirs the same way.
- Unexpected failures (a database or file error) keep their cause
  chain, which is what debugging needs. Wrap them with
  `errs.Context(err, "doing x")`, which leaves a reader's message from
  `errs.New` untouched, so every error can be wrapped the same way.
- `sdk.AuthorRequired(cmd)` is the one author-required sentence.
- `sdk.ExitError{Cmd, Code}` ends a command with a process exit code
  (`agent run` passes on the agent's). The CLI exits with the code and
  prints nothing; never call `os.Exit` from a handler, which skips the
  store's close and would end an MCP or HTTP server.
- Ctrl-C prints `llmd: interrupted.`
- The CLI prints the message as it is (no `error:` prefix). A missing
  argument prints the message, then the command's usage.

`TestErrorStyle` (`cli/errstyle_test.go`) runs failing commands and
checks every message against the style; `TestNoStackedErrors`
(`internal/errs/style_test.go`) rejects the source patterns that
stacked phrases. Add a case to the first when adding an error a user
can reach.

## Common Pitfalls

- **Version.Number** - `sdk.Version` uses `Number` for the 1-indexed
  version number field (not `Num`, not `Version`).

- **Task Move requires a spec** - Tasks cannot leave the backlog until
  their spec document has content beyond the title heading. `hasSpec()`
  in `internal/llmd/tasks/move.go` strips the first line (the `# Title`
  heading) and checks whether any non-whitespace content remains. A
  document with only `# Title` fails; use multi-line content like
  `[]byte("# Title\n\nBody content.")` in tests.

- **Import author** - The import bridge uses `origin("import")` for the author
  field. The internal validation requires a non-empty author.

- **Export appends .md** - Exported files get a `.md` extension. For batch
  export, the prefix must end with `/` (e.g. `"docs/"` not `"docs"`).

- **Remove returns errors for missing items** - `ctx.Links.Remove` and
  `ctx.Tags.Remove` return errors when the target link/tag does not exist.
  They are not idempotent.

- **Task path deduplication** - When `task add` is given no `--path`, it
  generates `tasks/<slug>`. If a document already exists there, or a live
  task already claims that path, it appends `-2`, `-3`, `-4` (incrementing)
  so tasks never share a spec and never silently version an unrelated
  document. Explicit `--path` skips deduplication.

- **--db name resolution** - The `--db` flag accepts a bare name (e.g. `docs`)
  which `path.ResolveDB` converts to `.llmd/llmd-docs.db`. A value with path
  separators or ending in `.db` is used as-is. Empty defaults to `.llmd/llmd.db`.
  `ResolveDB` returns `(string, error)` - shorthand names are sanitised (spaces
  become dashes, consecutive dashes collapse, leading/trailing dashes trimmed)
  and rejected if they contain control characters, Windows-illegal characters
  (`< > : " | ? *`), or path traversal (`..`). Explicit paths skip sanitisation.
  `ctx.Mirror.Directory()` returns the mirror directory for the active store.

- **Compiled extensions register in init()** - Missing a blank import in
  `main.go` silently omits all commands from that extension.
