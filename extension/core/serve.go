// serve.go implements the "llmd serve" command for MCP server operation.
//
// Separated from extension.go because serve has unique lifecycle requirements.
// Unlike other commands that run and exit, serve blocks indefinitely handling
// MCP requests over stdio.
//
// Design: Serve is a NoStoreCommand - it manages its own service lifecycle
// instead of using the shared service from root.go. This is necessary because
// serve needs to control when the database connection is opened and closed,
// rather than having it managed by the CLI framework.

package core

import (
	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/internal/mcp"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server",
		Long: `Start an MCP (Model Context Protocol) server over stdio for LLM integration.

Use --db to serve a specific database:
  llmd serve --db docs    # serve llmd-docs.db`,
		RunE: runServe,
	}
}

func runServe(_ *cobra.Command, _ []string) error {
	return mcp.Serve(cmd.DB())
}
