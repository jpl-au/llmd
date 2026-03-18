# Architecture Reference

## Running llmd

If `llmd` is not on your PATH, use the binary built in the project root
(`./llmd`). Try the PATH first; fall back to `./llmd` if not found.

## Core Principle

**The SDK is the single API surface. Everything goes through it.** CLI,
MCP, HTTP (coming), plugins, extensions — all are thin consumers that
call SDK interfaces. No domain logic lives in consumer layers.

This is deliberate: the API controls all actions and interactions through
a consistent code contract. Changes to internals are hidden behind the
SDK boundary and don't impact consumers. If something is available to
one consumer, it must be available to all of them through the SDK.

Every domain follows the same pattern:
1. Define the interface in `sdk/`
2. Implement in `internal/`
3. Bridge in `internal/host/`
4. Consumer layers (CLI, MCP, HTTP) are thin callers — no domain logic

Use structs-as-options for method parameters so that internal changes
stay hidden behind the SDK boundary.

**If logic is in a consumer layer and could be needed by another consumer,
it's in the wrong place.** Move it behind the SDK.

## Package Map

```
sdk/                        API surface: interfaces, types, flag parsing, globals
app/                        Build-time metadata: version tag, commit, build time
cli/                        Thin CLI dispatch: calls SDK, formats output
extension/                  Caddy-style compile-time plugin registry
internal/host/              SDK bridge: translates SDK calls to internal packages
internal/plugin/            Yaegi dynamic plugin loader
internal/llmd/              Store: opens database, coordinates sub-packages
internal/llmd/documents/    Document CRUD, versioning, soft-delete
internal/llmd/history/      Version listings, unified diffs, reverts
internal/llmd/search/       FTS5 full-text search and glob matching
internal/llmd/tasks/        Task board: columns, metadata, audit log
internal/llmd/tags/         Key-value tags attached to documents
internal/llmd/links/        Directed relationships between documents
internal/llmd/bulk/         Import from / export to filesystem (os.OpenRoot confined)
internal/llmd/events/       Internal synchronous event bus
internal/llmd/entities/     Named entity extraction
internal/llmd/audits/       Agent-to-agent and human-to-agent review threads
internal/llmd/audit/        Change log
internal/llmd/key/          ID generation: 9-char base36 from ms timestamps
internal/llmd/hash/         Content hashing (xxh3, blake2b)
internal/llmd/meta/         Document metadata helpers
internal/git/               Git CLI wrapper: sdk.GitStore implementation (build-tagged)
internal/config/            Configuration files and .llmd/.gitignore management
internal/line/              Platform-aware line ending conversion (build-tagged)
internal/validate/          Input validation: null bytes, path length, content size
internal/server/            HTTP API server: thin transport over sdk.Dispatch
internal/term/              Terminal detection and stdin reading utilities
internal/sql/               Schema migrations, SQL helpers
pkg/model/core/             Core model types (Origin)
pkg/events/                 Shared event type definitions
plugins/sample/             Example Yaegi plugin (stat, recent, wc)
guide/                      User-facing command guides (markdown)
```

## Command Flow

The CLI entry point is split across focused files in the `main` package:

- `main.go` — orchestration only (~114 lines)
- `flags.go` — `parseGlobal()` extracts global flags from args
- `output.go` — `display()` renders sdk.Response, `errorf()` formats errors
- `log.go` — `initLog()` configures slog
- `help.go` — `printHelp()`, `printCmdHelp()`

Terminal detection and stdin reading live in `internal/term/`.

```
main.go → run()
  ↓ parseGlobal(args) — flags.go
  ↓ config.Load() + initLog() — log.go
  ↓ signal.NotifyContext — Ctrl+C cancellation
  ↓ check extension.Storeless — skip store for init, version, config
  ↓ host.Open(dbPath) or host.New() depending on needsStore
  │   ├─ set sdk.Documents, sdk.Tasks, sdk.Links, sdk.Tags, sdk.Audits, sdk.Activities
  │   ├─ load compiled extensions via extension.All()
  │   ├─ wire extension EventHandlers to internal bus
  │   └─ load Yaegi plugins from .llmd/plugins/ and ~/.llmd/plugins/
  ↓ resolve author (flag → config → term.Interactive() fallback)
  ↓ term.ReadStdin() — internal/term/
  ↓ host.Exec(ctx, cmd, args, author, stdin, dbPath)
  │   └─ creates per-request bridge instances bound to ctx
  ↓ plugin.Exec(sctx, cmd, args) → sdk.Response
  ↓ display(result, jsonOut) — output.go
```

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

- `Flag` — describes one flag: `Name`, `Short` (single-char alias), `Type`
  (`"bool"`, `"string"`, `"int"`), `Desc`.
- `FlagValues` — returned by `ParseArgs`. Accessors: `Bool(name)`, `String(name)`,
  `Int(name)`, `Has(name)`. `Has` distinguishes "not provided" from "provided
  with zero value" (e.g. `--priority 0` vs omitted).
- `ParseArgs(flags []Flag, args []string) (FlagValues, []string, error)` — parses
  POSIX-style flags. Returns flag values and remaining positional arguments.

### Supported syntax

- `--name` (bool), `--name value`, `--name=value` (string/int)
- `-n` (short bool), `-lat` (combined short bools)
- `-C3` (compact short int), `-C 3` (separate short int)
- `-nC3` (combined: `n` is bool, `C` consumes `3`)
- `--` terminates flag parsing; everything after is positional

### Author enforcement on mixed commands

Commands that are purely mutating (write, edit, rm, mv, restore, revert, sed,
unlink, import) declare `NeedsAuthor: true` on the `sdk.Command`. The host
rejects these before dispatch if no author is set.

Mixed commands (tag, link, task) support both reads and writes. They do NOT
set `NeedsAuthor` — instead, their handlers check `ctx.Author == ""` on
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
| `sdk.GitStore` | `sdk/git.go` | `internal/git/git.go` (`Git`) | `internal/host/host.go` |
| `sdk.ConfigStore` | `sdk/config.go` | `internal/config/store.go` (`Store`) | `internal/host/host.go` |

Each bridge type in `internal/host/` holds a `context.Context` field and
translates SDK flat arguments into internal option structs, passing the
bound context to every internal store call. All mutating operations stamp
a `core.Origin{Source: "cli"}` for audit tracking.

**Context-Bound Bridges:** `host.Exec` creates fresh bridge instances per
command, each bound to the request's `context.Context`. This provides
cancellation and timeout support without changing any SDK interface
signatures. `sdk.Context` embeds `context.Context` and carries domain
store fields (`Documents`, `Tasks`, `Links`, `Tags`, `Activities`,
`Audits`, `Mirror`, `Git`, `Config`). CLI commands access stores via
`ctx.Documents`, `ctx.Tasks`, `ctx.Audits` rather than package globals.

Package globals (`sdk.Documents`, `sdk.Tasks`, `sdk.Audits`) remain wired with
`context.Background()` in `setup()`. They are still used by `sdk.Git` and
`sdk.Config` which have no per-request instances.

`sdk.GitStore` and `sdk.ConfigStore` have no host bridges — `internal/git.Git`
and `internal/config.Store` implement their interfaces directly. Both are
system utilities independent of the store, so bridges would add indirection
without value. They are shared via `sdk.Git`/`sdk.Config` globals rather
than per-request instances.

Bridge types also translate internal errors to SDK sentinels via per-domain
helpers (`docErr`, `taskErr`, `linkErr`, `tagErr`). This prevents internal
error types from leaking through the SDK boundary:

| Internal error | SDK sentinel |
|---------------|-------------|
| `documents.ErrNotFound` | `sdk.ErrNotFound` |
| `history.ErrNotFound` | `sdk.ErrNotFound` |
| `tasks.ErrNotFound` | `sdk.ErrNotFound` |
| `tasks.ErrNoSpec` | `sdk.ErrNoSpec` |
| `tasks.ErrMissingTitle` | `sdk.ErrMissingArg` |
| `tasks.ErrInvalidCol` | `sdk.ErrInvalidArg` |
| `links.ErrNotFound` | `sdk.ErrNotFound` |
| `links.ErrSelfLink` | `sdk.ErrInvalidArg` |
| `links.ErrExists` | `sdk.ErrExists` |
| `tags.ErrNotFound` | `sdk.ErrNotFound` |
| `tags.ErrInvalid` | `sdk.ErrInvalidArg` |
| `tags.ErrExists` | `sdk.ErrExists` |
| `audits.ErrNotFound` | `sdk.ErrNotFound` |
| `audits.ErrMissingAuthor` | `sdk.ErrMissingArg` |
| `audits.ErrMissingTarget` | `sdk.ErrMissingArg` |

CLI commands use context-local stores:

```go
ctx.Documents.Read("path", 0)
ctx.Tasks.Add("title", body, sdk.TaskAddOpts{Author: ctx.Author})
ctx.Tags.Add("path", "name", ctx.Author)
ctx.Links.Add("a", "b", "label", ctx.Author)
```

Yaegi plugins also use `sdk.Documents`, `sdk.Tasks`, `sdk.Audits` — but these
resolve through the Yaegi symbol table to per-adapter store fields, not
package globals. See the Plugin System section below.

## Plugin System

Two loading mechanisms, same `sdk.Plugin` interface:

**Compiled extensions** — registered at init-time via `extension.Register()`,
following the `database/sql.Register` pattern. The `cli` package uses this.
Panics on duplicate names to catch programmer errors early.

**Yaegi dynamic plugins** — Go source loaded at runtime without recompilation.
Discovered from `.llmd/plugins/<name>/` (project-local, takes priority) and
`~/.llmd/plugins/<name>/` (user-global). Each directory's `.go` files are
concatenated and evaluated by the Yaegi interpreter. The plugin must export
a `New()` function returning a value that satisfies `sdk.Plugin`. Yaegi
provides no security sandbox — plugins have full stdlib access and run with
the same permissions as the `llmd` process. Only run trusted plugins.

### Yaegi Symbol Table (`internal/plugin/symbols.go`)

The symbol table exports SDK types, constants, errors, domain stores, and
flag parsing (`FlagValues`, `ParseArgs`) so that interpreted plugin code can
`import "github.com/jpl-au/llmd/sdk"`.

Domain stores (`sdk.Documents`, `sdk.Tasks`, `sdk.Audits`) in the symbol table point
at per-adapter fields, not package-level globals. `load()` creates the adapter
before registering the symbol table so the reflect values are bound to adapter
fields from the start. `Exec` acquires the adapter mutex and populates those
fields from `ctx` before each call — giving each request its own request-scoped
stores. Plugin source is unchanged: `sdk.Documents.Read(...)` works
transparently regardless of how the underlying store is wired.

Interface wrappers (`_sdk_DocumentStore`, `_sdk_TaskStore`, `_sdk_AuditStore`)
exist because
Yaegi uses reflection to bridge interpreted types to Go interfaces. Each wrapper
struct has a `WMethodName` function field for every interface method. When Yaegi
needs to assign an interpreted struct to an interface variable, it populates
these fields with the plugin's method implementations, then the wrapper's
concrete methods delegate to the fields. When an interface changes, the wrapper
must be updated to match.

## Event System

Three separate event mechanisms serve different layers:

1. **Internal bus** (`internal/llmd/events/`) — synchronous, in-process. The
   FTS5 search handler subscribes to maintain the full-text index on document
   writes and deletes. Uses `pkg/events.Event` as the event type.

2. **Shared event types** (`pkg/events/`) — struct definitions for
   `document.written`, `document.deleted`, `document.restored`,
   `document.moved`. Consumed by the internal bus.

3. **Extension events** (`extension/events.go`) — fire-and-forget notifications
   for extensions. Eight event types: document write/delete/restore/move, tag
   add/remove, link create/remove. Extensions implement `EventHandler` to
   observe. The host bridges internal bus events to extension handlers via
   `internal/host/events.go`. Extensions cannot veto operations.

## Key Generation

`internal/llmd/key/` generates 9-character base36 identifiers from millisecond
timestamps plus an atomic counter. These are NOT nanoid. Format: lowercase
alphanumeric, lexicographically sortable by creation time. Used for all entity
IDs (documents, tasks, audits). **IDs are bare keys with no prefix** —
never add `aud_`, `task_`, `doc_`, or any other prefix.

The counter is seeded with a random offset in [0, 1000) at init to prevent
collisions between concurrent processes starting in the same millisecond.

## Testing

```bash
go test ./...
```

**Test setup** — `host.TestSetup(t, mode)` is the standard way to initialise
a store and wire SDK globals for testing. Two modes:

- `host.TestMemory` — in-memory store, fast, no disk I/O
- `host.TestDisk` — disk-backed store in a temp directory with chdir;
  use when tests need a real filesystem (e.g. git operations)

Host tests use `TestMemory`; CLI git tests use `TestDisk`. Plugin tests
cannot import host (import cycle) — they use stubs instead (see below).

**Plugin tests** (`internal/plugin/loader_test.go`) — use stub implementations
of all SDK store interfaces to avoid import cycles (`internal/host` imports
`internal/plugin`, so plugin tests cannot import host). Stubs are passed
directly via `sdk.Context` fields (`ctx.Documents`, `ctx.Tasks`, `ctx.Audits`) —
the same way the real host wires stores. Yaegi integration tests load real
`.go` source through the interpreter and verify that interpreted plugin code
can call methods on the domain stores.

## CLI Views

Three static terminal views provide human-friendly overviews. All use lipgloss
v2 (`charm.land/lipgloss/v2`) for styled output in terminals and fall back to
plain text when piped.

| Command | File | Description |
|---------|------|-------------|
| `diff` | `cli/diff.go` | Coloured unified diffs (green/red/cyan) when TTY |
| `status` | `cli/status.go` | Dashboard: recent docs, task summary, activity |
| `review` | `cli/review.go` | Pending tasks with spec previews and links |
| `ls --tree` | `cli/ls.go` | Directory hierarchy via `lipgloss/v2/tree` |

Shared styles live in `cli/styles.go`: table styles (`tblHeader`, `tblCell`,
`tblDim`, `tblBorder`) and diff colour styles (`diffAdded`, `diffRemoved`,
`diffHunk`, `diffHeader`). View-specific styles are local to each command file.

The `status` command uses a unified activity feed (`sdk.Activities.Recent()`)
that queries documents, entities (tags/links), and task audit events in parallel,
then merges by timestamp. The feed is defined as `sdk.ActivityStore` (in
`sdk/activity.go`), implemented in `internal/llmd/activity.go`, bridged through
`internal/host/api_activity.go` (`activityAPI`), and wired in `internal/host/host.go`.

Task audit actions use past tense: `"created"`, `"moved"`, `"deleted"`,
`"restored"`, `"edited:*"`, `"flagged"`, `"unflagged"`.

## Git-Aware Tasks

Tasks can be linked to git branches via the `Branch` field. Multi-step
orchestration (start, finish, branch creation) lives in the SDK so all
consumers (CLI, MCP, HTTP, plugins) can use it. The CLI provides thin
dispatch on top.

### SDK methods (`sdk.TaskStore`)

| Method | Description |
|--------|-------------|
| `Start(key, author, StartOpts)` | Move to column + record current git branch (git optional) |
| `StartBranch(key, author, StartBranchOpts)` | Create branch from title slug, checkout, record, move to column |
| `Finish(key, author, FinishOpts)` | Move to done + return `FinishResult` with git summary counts |
| `ByBranch(branch)` | Find task linked to a branch name |

Implementation lives in `internal/host/api_tasks.go`. The `branchSlug`
helper (title → git-safe branch name) is unexported in the same file.

### CLI subcommands

| Command | File | Description |
|---------|------|-------------|
| `task start <id>` | `cli/task_git.go` | Parse flags → `sdk.Tasks.Start()` → format text |
| `task finish [id]` | `cli/task_git.go` | Parse flags → `sdk.Tasks.Finish()` → format summary |
| `task branch <id>` | `cli/task_git.go` | Parse flags → `sdk.Tasks.StartBranch()` → format text |
| `task diff [id]` | `cli/task_git.go` | Resolve task + `sdk.Git.Diff()` → coloured output |
| `task files [id]` | `cli/task_git.go` | Resolve task + `sdk.Git.Files()` → file list |
| `task commits [id]` | `cli/task_git.go` | Resolve task + `sdk.Git.Commits()` → commit list |

Commands marked `[id]` auto-detect the task from the current git branch
when no ID is given (`sdk.Tasks.ByBranch` via the current branch).
`task show` displays ahead/behind counts when the task has a branch
and git is available.

All git operations degrade gracefully — `Start` and `Finish` work
without git (skipping branch recording and git summary respectively);
`StartBranch` requires git and returns a clear error if unavailable.
Default branch detection tries `origin/HEAD` first, then `main`, then
`master`; override with `--base`.

## Audit Threads

The audit domain (`sdk.AuditStore`) provides agent-to-agent and human-to-agent
review threads. Audits are **insert-only** — no record is ever updated. Thread
status is derived from the most recent entry.

**Data model:** A single `audits` table. Top-level audits target a document
path or task key. Replies reference a `parent_id`. The store resolves replies
to the top-level ancestor (no nested threads). `deleted_at` is the one
pragmatic mutation — a visibility flag for soft-delete.

**Author vs assignee:** `author` is who created the entry. `assignee` is who
needs to act on it. These are separate fields — an agent creates an audit
(`--author`) and directs it to another agent (`--assign`). The assignee
propagates through replies like status does; the effective assignee is from
the most recent entry. `audit status` filters by effective assignee.

**ID format:** Bare 9-char base36 key with no prefix (same generator as tasks
and all other entities). IDs are never prefixed — no `aud_`, `task_`, `doc_`.

**Target type inference:** The store determines `target_type` from the target
value. If it matches a valid 9-char base36 key, it's a task; otherwise it's
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
to avoid feedback loops. Wired in `Store.wire()`.

**Strict ordering:** `Ack` rejects if the key does not match the consumer's
oldest pending message (`ErrOrderViolation`).

**Directed vs broadcast:** Messages with `assigned_to` set are only visible
to that consumer. NULL means broadcast — all consumers see it.

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
operations use `qwr.TransactionFunc` — a callback-based pattern that
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

**FTS handler:** `onWrite` and `onMove` use `TransactionFunc` so the
FTS delete-then-insert is atomic. A crash cannot leave the index missing
a document.

**When to use `recordTx` vs `audit.Record`:** Use `recordTx` inside a
transaction to make audit records atomic with the surrounding operation.
Use `audit.Record` only for standalone operations that do not share a
transaction.

## MCP Server

The MCP server (`cli/mcp.go`) exposes llmd commands as MCP tools over stdio.
All tools share a single `toolInput` schema with `args`, `content`, and `author`
fields. The `author` field lets the LLM identify itself per-call — this is the
primary author source in MCP mode, since there is no interactive user.

If the LLM omits `author` and the command has `NeedsAuthor: true`, the server
rejects the call immediately. Mixed commands (tag, link, task) do not set
`NeedsAuthor` — their handlers check author on mutation paths only, so read
operations succeed without an author.

## HTTP Server

The HTTP server (`internal/server/`) exposes llmd commands as HTTP endpoints.
It follows the same pattern as the MCP server — a thin transport layer over
`sdk.Dispatch`. Commands are registered automatically by walking
`sdk.AllCommands()`, so plugins get HTTP routes for free.

**Route structure:** URL paths mirror CLI commands. `/cat/docs/api` dispatches
to the `cat` command with path `docs/api`. `/grep?q=hello` dispatches to
`grep` with the search query as a positional argument.

**Method mapping:** Read commands (`NeedsAuthor == false`) are GET, mutation
commands are POST. The same `NeedsAuthor` field drives both MCP tool
classification and HTTP method selection.

**Headers carry metadata:** `Author` (fallback identity), `Message` (commit
message), `Source` (origin tracking), `Output` (`json` to request structured
data). No `X-` prefix — RFC 6648 deprecated it.

**Request body is content:** POST bodies carry raw document content (markdown),
not JSON envelopes. This matches the CLI where stdin is the document body.

**Response format:** `sdk.Text` returns `text/plain`. `sdk.Data` returns JSON.
`sdk.Result` returns text by default, or JSON when the `Output: json` header
is set or when no text representation exists.

**Error mapping:** SDK sentinel errors map to HTTP status codes —
`ErrNotFound` → 404, `ErrMissingArg`/`ErrInvalidArg` → 400, `ErrExists` → 409,
`ErrNoSpec` → 422. All errors return JSON `{"error": "..."}`.

**Configuration:** Listen address is read from `serve_addr` config key,
defaulting to `localhost:5563`. No flags or environment variables.

**Skipped commands:** `mcp`, `serve`, `init`, `config`, `version`, `plugins`,
`guide`, `llm` are not exposed over HTTP — they are admin or local-only.

The server uses `github.com/jpl-au/chain` as the HTTP mux, which wraps
Go 1.22's enhanced `net/http` routing with middleware support.

## Gitignore Management

llmd manages `.llmd/.gitignore` using a whitelist approach: everything
is ignored by default, and specific files are allowed through with `!`
patterns. This means new files (telemetry logs, temp files, mirror
output) are automatically excluded without maintaining a growing
blocklist. llmd never touches the project's root `.gitignore`.

**Default `.llmd/.gitignore` (created by `llmd init`):**
```
*
!*.db
!.gitignore
```

Everything is ignored except database files and the gitignore itself.
The database (`llmd.db`) is committed by default — shared context is
the intended use case.

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
- `log_level` — `debug`, `info`, `warn`, `error`
- `log_format` — `text`, `json`

`--json` implies `log_format=json` so structured output stays
machine-readable. `--verbose` overrides any configured level.

All packages use the process-wide `slog` default — call `slog.Debug`,
`slog.Info`, `slog.Warn`, `slog.Error` directly. No logger is passed
through context or structs.

## Common Pitfalls

- **Version.Number** — `sdk.Version` uses `Number` for the 1-indexed
  version number field (not `Num`, not `Version`).

- **Task Move requires a spec** — Tasks cannot leave the backlog until
  their spec document has content beyond the title heading. `hasSpec()`
  in `internal/llmd/tasks/move.go` strips the first line (the `# Title`
  heading) and checks whether any non-whitespace content remains. A
  document with only `# Title` fails; use multi-line content like
  `[]byte("# Title\n\nBody content.")` in tests.

- **Import author** — The import bridge uses `origin("import")` for the author
  field. The internal validation requires a non-empty author.

- **Export appends .md** — Exported files get a `.md` extension. For batch
  export, the prefix must end with `/` (e.g. `"docs/"` not `"docs"`).

- **Remove returns errors for missing items** — `sdk.Links.Remove` and
  `sdk.Tags.Remove` return errors when the target link/tag does not exist.
  They are not idempotent.

- **Task path deduplication** — When `task add` creates a spec document
  automatically (no `--path`), it generates `tasks/<slug>`. If that path
  already exists, it appends `-2`, `-3`, `-4` (incrementing) to avoid silently versioning
  an unrelated document. Explicit `--path` skips deduplication.

- **--db name resolution** — The `--db` flag accepts a bare name (e.g. `docs`)
  which `path.ResolveDB` converts to `.llmd/llmd-docs.db`. A value with path
  separators or ending in `.db` is used as-is. Empty defaults to `.llmd/llmd.db`.
  `ResolveDB` returns `(string, error)` — shorthand names are sanitised (spaces
  become dashes, consecutive dashes collapse, leading/trailing dashes trimmed)
  and rejected if they contain control characters, Windows-illegal characters
  (`< > : " | ? *`), or path traversal (`..`). Explicit paths skip sanitisation.
  `sdk.Mirror.Directory()` returns the mirror directory for the active store.

- **Import cycle: host ↔ plugin** — `internal/host` imports `internal/plugin`.
  Plugin tests cannot import host. Use stub implementations instead.

- **Compiled extensions register in init()** — Missing a blank import in
  `main.go` silently omits all commands from that extension.
