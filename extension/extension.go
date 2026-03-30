// Package extension provides the extension architecture for llmd.
// Extensions encapsulate related functionality (commands, MCP tools) and
// register at init time, enabling modular feature development without
// touching core code.
//
// Extensions implement [sdk.Extension] directly for command dispatch.
// Optional lifecycle interfaces (Initializable, Vacuumable, EventHandler)
// are checked via type assertion by the host.
package extension

import (
	"time"
)

// Initializable extensions can perform setup (migrations, etc).
type Initializable interface {
	Init(ctx Context) error
}

// Storeless is an optional interface for extensions with commands that
// don't require a store. Commands returned by NoStoreCommands() will
// not trigger store initialisation.
//
// Use cases:
// 1. Bootstrap commands (like init) that run before store exists
// 2. Commands that manage their own service lifecycle
// 3. Utility commands that don't need document storage
type Storeless interface {
	NoStoreCommands() []string
}

// Vacuumable extensions can clean up their own soft-deleted data.
// The vacuum command calls Vacuum on all extensions implementing this
// interface after vacuuming core tables. This allows extensions with
// custom tables to participate in the cleanup process.
//
// Extensions use ctx.DB() to run their own vacuum SQL on their own tables.
type Vacuumable interface {
	// Vacuum permanently deletes soft-deleted records older than the given duration.
	// If olderThan is nil, all soft-deleted records are removed.
	// Returns the count of records deleted.
	Vacuum(ctx Context, olderThan *time.Duration) (int64, error)
}
