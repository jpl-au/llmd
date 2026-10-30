// Package harness holds the harnesses that run agent programs for llmd, and llmd's own harness, named llmd.
//
// A harness implements [sdk.Harness]. A build of llmd that brings its own harness registers it in init and builds its binary as the cmd package shows:
//
//	func init() { harness.Register(myHarness{}) }
//
// An agent runs under the harness the store names for it (llmd agent harness), unless agent.harness in the configuration file names the harness this computer runs. An agent that has neither runs under llmd's own.
package harness

import (
	"slices"
	"sync"

	"github.com/jpl-au/llmd/sdk"
)

// Default is the name of llmd's own harness.
const Default = "llmd"

var (
	mu        sync.RWMutex
	harnesses = make(map[string]sdk.Harness)
)

func init() {
	Register(llmd{})
}

// Register makes a harness available by its name. It is called from init. A second harness of the same name panics, as database/sql.Register does: the name is a mistake in the build, not a condition at run time.
func Register(h sdk.Harness) {
	mu.Lock()
	defer mu.Unlock()
	name := h.Name()
	if _, ok := harnesses[name]; ok {
		panic("harness already registered: " + name)
	}
	harnesses[name] = h
}

// Lookup returns the harness registered under name.
func Lookup(name string) (sdk.Harness, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := harnesses[name]
	return h, ok
}

// Names returns the names of the registered harnesses in order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(harnesses))
	for name := range harnesses {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
