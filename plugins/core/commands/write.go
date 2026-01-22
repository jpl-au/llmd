//go:build wasip1

// This file implements the write command for creating/updating documents.
package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Write defines the write command for creating or updating documents.
//
// The write command reads content from the execution context (piped from stdin
// for CLI, or from tool parameters for MCP) and writes it to the specified
// path. This creates a new version of the document, preserving history.
var Write = sdk.Command{
	Name:        "write",
	Description: "Write a document",
	Usage:       "write <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "message", Short: "m", Type: "string", Description: "Version message"},
	},
}

// ExecWrite executes the write command.
//
// Reads content from ctx.Stdin (piped from CLI or passed from MCP) and writes
// it to the document at the specified path. The author from the execution
// context is recorded in version history. An optional message flag can describe
// the change.
func ExecWrite(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("write: missing path argument")
	}

	path := args[0]

	message, _ := flags["message"].(string)

	// Read content from context (provided by host from stdin or MCP params)
	content := ctx.Stdin

	if err := sdk.Host.Write(path, content, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Wrote %s", path)), nil
}
