# Architecture Reference

## Package Map

```
sdk/                        Plugin SDK: interfaces, types, globals
cli/                        Core commands (cat, ls, write, rm, task, status, review, etc.)
extension/                  Caddy-style compile-time plugin registry
internal/host/              Plugin host: discovery, dispatch, SDK bridge
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
internal/llmd/audit/        Change log
internal/llmd/key/          ID generation: 9-char base36 from ms timestamps
internal/llmd/hash/         Content hashing (xxh3, blake2b)
internal/llmd/meta/         Document metadata helpers
internal/config/            Configuration files and .llmd/.gitignore management
internal/line/              Platform-aware line ending conversion (build-tagged)
internal/validate/          Input validation: null bytes, path length, content size
internal/sql/               Schema migrations, SQL helpers
pkg/model/core/             Core model types (Origin, etc.)
pkg/events/                 Shared event type definitions
plugins/sample/             Example Yaegi plugin (stat, recent, wc)
guide/                      User-facing command guides (markdown)
```

## Command Flow

```
main.go
  ↓ parse global flags (--json, --help, --db)
  ↓ check extension.Storeless — skip store for init, version, config
  ↓ host.Open(dbPath) or host.New() depending on needsStore
  │   ├─ set sdk.Documents, sdk.Tasks, sdk.Links, sdk.Tags, sdk.Activities
  │   ├─ load compiled extensions via extension.All()
  │   ├─ wire extension EventHandlers to internal bus
  │   └─ load Yaegi plugins from .llmd/plugins/ and ~/.llmd/plugins/
  ↓ host.Exec(cmd, args, author, stdin, dbPath)
  ↓ plugin.Exec(ctx, cmd, args) → sdk.Response
  ↓ type-switch on Response: Text → print, Data → JSON, Result → text or JSON
```

## Domain Interfaces

Four focused interfaces replace the old monolithic `Store`:

| Interface | Defined in | Implemented by | Wired in |
|-----------|-----------|----------------|----------|
| `sdk.DocumentStore` | `sdk/documents.go` | `internal/host/api_documents.go` (`documentAPI`) | `internal/host/host.go` |
| `sdk.TaskStore` | `sdk/tasks.go` | `internal/host/api_tasks.go` (`taskAPI`) | `internal/host/host.go` |
| `sdk.LinkStore` | `sdk/links.go` | `internal/host/api_links.go` (`linkAPI`) | `internal/host/host.go` |
| `sdk.TagStore` | `sdk/tags.go` | `internal/host/api_tags.go` (`tagAPI`) | `internal/host/host.go` |
| `sdk.ActivityStore` | `sdk/activity.go` | `internal/host/api_activity.go` (`activityAPI`) | `internal/host/host.go` |

Each bridge type in `internal/host/` translates SDK flat arguments into
internal option structs and maps internal results back to SDK types. All
mutating operations stamp a `core.Origin{Source: "cli"}` for audit tracking.

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

Plugins call these through globals:

```go
sdk.Documents.Read("path", 0)
sdk.Tasks.Add("title", body, sdk.TaskAddOpts{Author: ctx.Author})
sdk.Tags.Add("path", "name", ctx.Author)
sdk.Links.Add("a", "b", "label", ctx.Author)
```

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

The symbol table exports SDK globals, types, constants, and errors so that
interpreted plugin code can `import "github.com/jpl-au/llmd/sdk"`.

Interface wrappers (`_sdk_DocumentStore`, `_sdk_TaskStore`, etc.) exist because
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
alphanumeric, lexicographically sortable by creation time. Used for document
keys, task keys, and other entities.

The counter is seeded with a random offset in [0, 1000) at init to prevent
collisions between concurrent processes starting in the same millisecond.

## Testing

```bash
go test ./...
```

**Host tests** (`internal/host/api*_test.go`) — use `testHost(t)` which opens
an in-memory SQLite store, sets SDK globals, and cleans up after the test.

**Plugin tests** (`internal/plugin/loader_test.go`) — use stub implementations
of all four SDK interfaces to avoid import cycles (`internal/host` imports
`internal/plugin`, so plugin tests cannot import host). The `wireStubs(t)`
helper sets SDK globals to stubs and restores them on cleanup. Yaegi
integration tests load real `.go` source through the interpreter and verify
that interpreted plugin code can call methods on domain globals.

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

Tasks can be linked to git branches via the `Branch` field. Six CLI
subcommands provide the integration:

| Command | File | Description |
|---------|------|-------------|
| `task start <id>` | `cli/task_git.go` | Move to in-progress + record branch (git optional) |
| `task finish [id]` | `cli/task_git.go` | Move to done + show summary (files, commits) |
| `task branch <id>` | `cli/task_git.go` | Create branch from task slug, checkout, start |
| `task diff [id]` | `cli/task_git.go` | Git diff for task's branch vs default branch |
| `task files [id]` | `cli/task_git.go` | List changed files on task's branch |
| `task commits [id]` | `cli/task_git.go` | List commits on task's branch |

Commands marked `[id]` auto-detect the task from the current git branch
when no ID is given (`taskForBranch` in `cli/task_git.go` matches the
current branch against tasks via indexed `Branch` filter). `task show` displays ahead/behind
counts when the task has a branch and git is available.

Git operations live in the CLI layer only — the SDK and backend just store
the branch string. Low-level helpers in `cli/git.go` shell out to `git`
via `os/exec`:

| Helper | Description |
|--------|-------------|
| `gitAvailable` | Checks git is installed and we're in a repo |
| `gitBranch` | Current branch name |
| `gitDefaultBranch` | Detects default branch (origin/HEAD, then main/master) |
| `gitDiff` | Three-dot diff between branches |
| `gitFiles` | Changed files between branches |
| `gitCommits` | Commit log between branches |
| `gitCheckoutNew` | Create and switch to new branch |
| `gitRevCount` | Ahead/behind commit counts |

All git subcommands degrade gracefully — `gitAvailable()` is checked
first and returns a clear error if git is missing or we're not in a repo.
`task start` and `task finish` work without git (skipping branch recording
and git summary respectively); other git commands return the error.
Default branch detection tries `origin/HEAD` first, then `main`, then
`master`; override with `--base`.

## Transaction Patterns

All multi-statement write operations wrap their queries in a single
`sql.Tx` so a crash cannot leave data partially updated.

**Documents:** `Write` delegates to `writeInTx` inside a `BeginTx`/`Commit`
pair. The event bus fires *after* commit so subscribers see committed data.
`writeInTx` and `readInTx` exist for callers that need document operations
within a larger transaction.

**Tasks:** `Add`, `Move`, `Set`, `Delete`, and `Restore` each open a
transaction, perform their UPDATE/INSERT, write the audit record via
`recordTx`, and commit. `recordTx` (in `helpers.go`) mirrors `audit.Log.Record`
but writes on the provided `*sql.Tx`. `repositionTx` (in `move.go`) renumbers
column positions within a transaction.

`Tasks.ensure()` calls `audit.Log.Ensure()` to guarantee the history table
exists before any `recordTx` call. `audit.Log.Ensure` is exported and
idempotent (guarded by `sync.Once`).

**Links:** `Remove` wraps its soft-delete loop in a transaction so multiple
link deletions are atomic.

**When to use `recordTx` vs `audit.Record`:** Use `recordTx` inside a
transaction to make audit records atomic with the surrounding operation.
Use `audit.Record` only for standalone operations that do not share a
transaction.

## MCP Server

The MCP server (`cli/mcp.go`) exposes llmd commands as MCP tools over stdio.
All tools share a single `toolInput` schema with `args`, `content`, and `author`
fields. The `author` field lets the LLM identify itself per-call — this is the
primary author source in MCP mode, since there is no interactive user.

If the LLM omits `author`, the server falls back to the configured CLI author.
If neither is set and the command has `NeedsAuthor: true`, `host.Exec()` rejects
the call with `sdk.ErrMissingArg`.

## Gitignore Management

llmd manages `.llmd/.gitignore` to exclude ephemeral files from version
control. It never touches the project's root `.gitignore`.

**Default `.llmd/.gitignore` (created by `llmd init`):**
```
*.db-wal
*.db-shm
llmd/
```

SQLite temp files and the default mirror directory are always ignored.
The database (`llmd.db`) is committed by default — shared context is
the intended use case. Users who want local-only stores can add the
database to the ignore list.

**Mirror auto-registration:** `llmd mirror` adds its output directory
(e.g. `llmd/`, `llmd-docs/`) to the gitignore if not already present.

**CLI management:**
```bash
llmd config ignore ls              # list patterns
llmd config ignore add "*.db"      # ignore all databases
llmd config ignore rm "*.db"       # remove pattern
```

## Common Pitfalls

- **Version.Number** — `sdk.Version` uses `Number` for the 1-indexed
  version number field (not `Num`, not `Version`).

- **Task Move requires a spec** — Tasks in backlog cannot move to other
  columns without a spec. `hasSpec()` in `internal/llmd/tasks/` strips the
  first line (the heading) and checks whether content remains. A single-line
  body like `[]byte("has spec")` has no spec; use multi-line like
  `[]byte("# Title\n\nBody content.")`.

- **Import author** — The import bridge uses `origin("import")` for the author
  field. The internal validation requires a non-empty author.

- **Export appends .md** — Exported files get a `.md` extension. For batch
  export, the prefix must end with `/` (e.g. `"docs/"` not `"docs"`).

- **Remove returns errors for missing items** — `sdk.Links.Remove` and
  `sdk.Tags.Remove` return errors when the target link/tag does not exist.
  They are not idempotent.

- **Task path deduplication** — When `task add` creates a spec document
  automatically (no `--path`), it generates `tasks/<slug>`. If that path
  already exists, it appends `-2`, `-3`, etc. to avoid silently versioning
  an unrelated document. Explicit `--path` skips deduplication.

- **--db name resolution** — The `--db` flag accepts a bare name (e.g. `docs`)
  which `path.ResolveDB` converts to `.llmd/llmd-docs.db`. A value with path
  separators or ending in `.db` is used as-is. Empty defaults to `.llmd/llmd.db`.
  `ResolveDB` returns `(string, error)` — shorthand names are sanitised (spaces
  become dashes, consecutive dashes collapse, leading/trailing dashes trimmed)
  and rejected if they contain control characters, Windows-illegal characters
  (`< > : " | ? *`), or path traversal (`..`). Explicit paths skip sanitisation.
  `MirrorDir` derives the mirror directory from a db path, propagating the error.

- **Import cycle: host ↔ plugin** — `internal/host` imports `internal/plugin`.
  Plugin tests cannot import host. Use stub implementations instead.

- **Compiled extensions register in init()** — Missing a blank import in
  `main.go` silently omits all commands from that extension.
