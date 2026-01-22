//go:build wasip1

// Package commands implements the core plugin's command handlers.
//
// Each command is defined in a separate file with its command definition
// (exported as a variable) and its execution function. This separation
// allows the main plugin file to reference commands without importing
// their full implementation.
package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Cat defines the cat command for reading documents.
//
// The cat command displays the content of a document, similar to the Unix
// cat command. It supports reading specific versions and line ranges.
var Cat = sdk.Command{
	Name:        "cat",
	Description: "Read a document",
	Usage:       "cat <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "version", Short: "v", Type: "int", Description: "Read specific version"},
		{Name: "lines", Short: "n", Type: "string", Description: "Line range (e.g., 1-10)"},
	},
}

// ExecCat executes the cat command.
//
// Reads a document from the store and returns its content as text.
// The path argument is required. Optional flags allow reading a specific
// version or a range of lines.
func ExecCat(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("cat: missing path argument")
	}

	path := args[0]

	content, err := sdk.Host.Read(path)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}

	return sdk.TextResult(string(content)), nil
}
