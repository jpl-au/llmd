// Package all imports all core llmd extensions.
// Import this package to register all built-in commands.
package all

import (
	// Core extensions - each registers itself via init()
	_ "github.com/jpl-au/llmd/extension/core"
	_ "github.com/jpl-au/llmd/extension/document"
	_ "github.com/jpl-au/llmd/extension/edit"
	_ "github.com/jpl-au/llmd/extension/link"
	_ "github.com/jpl-au/llmd/extension/search"
	_ "github.com/jpl-au/llmd/extension/sync"
	_ "github.com/jpl-au/llmd/extension/tag"
)
