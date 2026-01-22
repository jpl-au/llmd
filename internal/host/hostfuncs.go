// This file defines the HostFuncs type that implements host functions for plugins.
//
// Host functions are exported to plugins via the "env" WebAssembly module. When a
// plugin calls sdk.Host.Read(), that call goes through the proto-generated client
// code, which invokes the corresponding host function, which ends up calling the
// appropriate method on HostFuncs.
//
// The actual method implementations are split across multiple files by domain:
//   - documents.go: Document operations (read, write, edit, delete, etc.)
//   - search.go: Search operations (full-text, regex, glob)
//   - history.go: Version history operations (list, diff, revert)
//   - tags.go: Tag operations (add, remove, list)
//   - links.go: Link operations (add, remove, list)
//   - entities.go: Entity operations (read, write, delete, list)
//   - events.go: Event operations (subscribe, emit)
package host

import (
	"github.com/jpl-au/llmd/internal/llmd"
)

// HostFuncs implements the host.Host interface from the proto-generated code.
//
// This type bridges the gap between the plugin's WASM sandbox and the llmd
// document store. Each method validates input, performs the requested operation
// on the store, and returns the result in the expected proto format.
//
// HostFuncs is instantiated once per Runtime and shared across all loaded plugins.
// All methods are safe for concurrent use.
type HostFuncs struct {
	// store is the llmd document store that plugins access.
	store *llmd.Store
}

// NewHostFuncs creates a new HostFuncs instance backed by the given store.
//
// The HostFuncs is passed to hostpb.Instantiate() to make the host functions
// available to plugins.
func NewHostFuncs(store *llmd.Store) *HostFuncs {
	return &HostFuncs{store: store}
}
