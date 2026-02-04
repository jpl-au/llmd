// registry.go implements the extension registration system.
//
// Separated from extension.go to isolate the global registry state and
// thread-safe access patterns. Extensions self-register during init(),
// before main() runs.
//
// Design: The registry uses panic-on-duplicate following database/sql.Register
// conventions. This catches programmer errors early rather than allowing
// silent failures. Registration order is preserved to ensure deterministic
// command ordering across runs.

package extension

import "sync"

// Registry holds all registered extensions.
var (
	mu       sync.RWMutex
	registry = make(map[string]Extension)
	order    []string // preserve registration order
)

// Register adds an extension to the registry. Called from init() functions.
//
// Why panic instead of returning error: Registration happens at init time,
// before main() runs. Errors at this stage indicate programmer mistakes
// (duplicate extension names), not runtime conditions. Panicking:
// 1. Fails fast and loudly during development
// 2. Avoids needing error handling in every init()
// 3. Makes duplicate registration impossible to ignore
//
// This follows the pattern used by database/sql.Register, flag.Var, etc.
func Register(e Extension) {
	mu.Lock()
	defer mu.Unlock()

	name := e.Name()
	if _, exists := registry[name]; exists {
		panic("extension already registered: " + name)
	}

	registry[name] = e
	order = append(order, name)
}

// All returns all registered extensions in registration order.
func All() []Extension {
	mu.RLock()
	defer mu.RUnlock()

	exts := make([]Extension, 0, len(order))
	for _, name := range order {
		exts = append(exts, registry[name])
	}
	return exts
}

// Get returns a specific extension by name, or nil if not found.
func Get(name string) Extension {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}

// Names returns the names of all registered extensions.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, len(order))
	copy(names, order)
	return names
}
