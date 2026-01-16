// Package document provides the document extension for core CRUD operations.
// Registers commands: cat, ls, write, rm, restore, revert, mv, history, diff.
//
// These commands mirror Unix filesystem utilities to provide familiar semantics
// for LLM and human users. Each command file is separated to isolate its
// specific flag handling and output formatting logic.

package document

import (
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	extension.Register(&Extension{})
}

// Extension implements the document extension.
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

// Name returns "document" - this extension handles core document CRUD operations.
func (e *Extension) Name() string { return "document" }

// Init connects to the shared service for document operations.
func (e *Extension) Init(ctx extension.Context) error {
	e.svc = ctx.Service()
	e.cfg = ctx.Config()
	return nil
}

// Commands returns Unix-like document manipulation commands.
func (e *Extension) Commands() []*cobra.Command {
	return []*cobra.Command{
		e.newCatCmd(),
		e.newLsCmd(),
		e.newWriteCmd(),
		e.newRmCmd(),
		e.newRestoreCmd(),
		e.newRevertCmd(),
		e.newMvCmd(),
		e.newHistoryCmd(),
		e.newDiffCmd(),
	}
}

// MCPTools returns nil - document MCP tools are provided by internal/mcp package.
func (e *Extension) MCPTools() []extension.MCPTool {
	return nil
}
