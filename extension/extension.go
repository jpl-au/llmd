// Package extension provides the plugin architecture for llmd. Extensions
// encapsulate related functionality (commands, MCP tools) and register at
// init time, enabling modular feature development without touching core code.
//
// Extensions wrap sdk.Plugin instances to leverage the existing command dispatch
// mechanism while adding lifecycle hooks (Init, Vacuum) and framework features
// (compile-time registration, event handling).
package extension

import (
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// Extension defines the contract for llmd extensions.
type Extension interface {
	// Name returns a unique identifier for this extension.
	Name() string

	// Plugin returns the sdk.Plugin for host registration.
	// This provides commands and tools via the existing dispatch mechanism.
	Plugin() sdk.Plugin
}

// Initializable extensions can perform setup (migrations, etc).
type Initializable interface {
	Extension
	Init(ctx Context) error
}

// Storeless is an optional interface for extensions with commands that
// don't require a store. Commands returned by NoStoreCommands() will
// not trigger store initialisation in PersistentPreRunE.
//
// Use cases:
// 1. Bootstrap commands (like init) that run before store exists
// 2. Commands that manage their own service lifecycle
// 3. Utility commands that don't need document storage
type Storeless interface {
	NoStoreCommands() []string
}

// Vacuumable extensions can clean up their own soft-deleted data.
// The vacuum command calls Vacuum on all extensions implementing this interface
// after vacuuming core tables. This allows extensions with custom tables
// (e.g., tasks, comments) to participate in the cleanup process.
//
// Extensions use ctx.DB() to run their own vacuum SQL on their own tables.
type Vacuumable interface {
	Extension
	// Vacuum permanently deletes soft-deleted records older than the given duration.
	// If olderThan is nil, all soft-deleted records are removed.
	// Returns the count of records deleted.
	Vacuum(ctx Context, olderThan *time.Duration) (int64, error)
}
