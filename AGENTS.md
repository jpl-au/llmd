# Architecture Reference

## Package Map

```
sdk/                        Plugin SDK: interfaces, types, globals
cli/                        Core commands (cat, ls, write, rm, task, etc.)
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
internal/llmd/bulk/         Import from / export to filesystem
internal/llmd/events/       Internal synchronous event bus
internal/llmd/entities/     Named entity extraction
internal/llmd/audit/        Change log
internal/llmd/key/          ID generation: 9-char base36 from ms timestamps
internal/llmd/hash/         Content hashing (xxh3, blake2b)
internal/llmd/meta/         Document metadata helpers
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
  ↓ llmd.Open(path) or llmd.Init(path) if store needed
  ↓ host.New(store)
  │   ├─ set sdk.Documents, sdk.Tasks, sdk.Links, sdk.Tags
  │   ├─ load compiled extensions via extension.All()
  │   └─ load Yaegi plugins from .llmd/plugins/ and ~/.llmd/plugins/
  ↓ host.Exec(cmd, args, author, stdin, dbPath)
  ↓ plugin.Exec(ctx, cmd, args) → sdk.Response
  ↓ type-switch on Response: Text → print, Data → JSON, Result → text or JSON
```

## Domain Interfaces

Four focused interfaces replace the old monolithic `Store`:

| Interface | Defined in | Implemented by | Wired in |
|-----------|-----------|----------------|----------|
| `sdk.DocumentStore` | `sdk/documents.go` | `internal/host/api.go` (`documentAPI`) | `internal/host/host.go` |
| `sdk.TaskStore` | `sdk/tasks.go` | `internal/host/api_tasks.go` (`taskAPI`) | `internal/host/host.go` |
| `sdk.LinkStore` | `sdk/links.go` | `internal/host/api_links.go` (`linkAPI`) | `internal/host/host.go` |
| `sdk.TagStore` | `sdk/tags.go` | `internal/host/api_tags.go` (`tagAPI`) | `internal/host/host.go` |

Each bridge type in `internal/host/` translates SDK flat arguments into
internal option structs and maps internal results back to SDK types. All
mutating operations stamp a `core.Origin{Source: "cli"}` for audit tracking.

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
a `New()` function returning a value that satisfies `sdk.Plugin`.

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
   for extensions. Seven event types: document write/delete/restore, tag
   add/remove, link create/remove. Extensions implement `EventHandler` to
   observe. Extensions cannot veto operations.

## Key Generation

`internal/llmd/key/` generates 9-character base36 identifiers from millisecond
timestamps plus an atomic counter. These are NOT nanoid. Format: lowercase
alphanumeric, lexicographically sortable by creation time. Used for document
keys, task keys, and other entities.

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

- **Import cycle: host ↔ plugin** — `internal/host` imports `internal/plugin`.
  Plugin tests cannot import host. Use stub implementations instead.

- **Compiled extensions register in init()** — Missing a blank import in
  `main.go` silently omits all commands from that extension.
