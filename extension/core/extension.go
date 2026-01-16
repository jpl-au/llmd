// Package core provides the core extension for llmd.
// It registers commands: init, config, serve, guide, vacuum, llm, db.
package core

import (
	"github.com/jpl-au/llmd/extension"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the core extension.
type Extension struct{}

// Compile-time interface compliance. Catches missing methods at build time
// rather than runtime, making interface changes safer to refactor.
var (
	_ extension.Extension = (*Extension)(nil)
	_ extension.Storeless = (*Extension)(nil)
)

// Name returns "core" - this extension provides fundamental llmd commands.
func (e *Extension) Name() string { return "core" }

// Commands returns all core CLI commands for repository management.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		newInitCmd(),
		newConfigCmd(),
		newServeCmd(),
		newGuideCmd(),
		newVacuumCmd(),
		newLlmCmd(),
		newDBCmd(),
		newVersionCmd(),
	}
}

// MCPTools returns nil - core commands have no MCP tool equivalents.
// MCP tools are provided by other extensions (document, search, etc.).
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}

// NoStoreCommands returns commands that manage their own service lifecycle.
// serve: Long-running MCP server needs its own service lifecycle.
// vacuum: Must work with --dry-run without requiring a store.
// db: Manages gitignore, doesn't need database connection.
// version: Displays build info, doesn't need database connection.
func (e *Extension) NoStoreCommands() []string {
	return []string{"serve", "vacuum", "db", "version"}
}
