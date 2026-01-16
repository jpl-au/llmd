// Package search provides document discovery and content searching.
// Supports FTS5 full-text search, regex matching, and glob patterns.
// Registers commands: find, grep, glob.
package search

import (
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the search extension.
type Extension struct {
	svc service.Service
	cfg *config.Config
}

// Compile-time interface compliance. Catches missing methods at build time
// rather than runtime, making interface changes safer to refactor.
var (
	_ extension.Extension     = (*Extension)(nil)
	_ extension.Initializable = (*Extension)(nil)
)

// Name returns "search" - this extension provides document discovery commands.
func (e *Extension) Name() string { return "search" }

// Init connects to the shared service for search operations.
func (e *Extension) Init(ctx extension.Context) error {
	e.svc = ctx.Service()
	e.cfg = ctx.Config()
	return nil
}

// Commands returns find, grep, and glob commands for document discovery.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		e.newFindCmd(),
		e.newGrepCmd(),
		e.newGlobCmd(),
	}
}

// MCPTools returns nil - MCP search tools are in internal/mcp.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}
