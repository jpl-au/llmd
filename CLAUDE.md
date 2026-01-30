# Build

Build the project (plugins + host):

```bash
go run tools/build/main.go
```

This compiles the WASM plugin first, then the host binary. Do not run `go build` directly - the plugin must be built first.

Individual targets:

```bash
go run tools/build/main.go plugins  # plugins only
go run tools/build/main.go host     # host only (requires plugins built)
```

# Test

```bash
go test ./...
```

# Architecture

- `plugins/core/` - WASM plugin with commands (cat, ls, write, rm, edit, mv, grep, etc.)
- `internal/llmd/` - document store backend
- `internal/llmd/search/` - FTS5 full-text search and path glob matching
- `internal/llmd/events/` - event bus for FTS index maintenance
- `sdk/` - plugin SDK (builds with GOOS=wasip1)

# Flag Conventions

Plugin commands should use long-form flags (e.g., `--version` not `-v`)
to avoid conflicts with Unix commands.
