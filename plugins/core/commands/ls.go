//go:build wasip1

// This file implements the ls command for listing documents.
package commands

import (
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Ls defines the ls command for listing documents.
//
// The ls command lists documents in the store, similar to the Unix ls command.
// It supports filtering by prefix and various display formats.
var Ls = sdk.Command{
	Name:        "ls",
	Description: "List documents",
	Usage:       "ls [prefix]",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "long", Short: "l", Type: "bool", Description: "Long format with details"},
		{Name: "all", Short: "a", Type: "bool", Description: "Include deleted documents"},
		{Name: "recursive", Short: "r", Type: "bool", Description: "List recursively"},
	},
}

// ExecLs executes the ls command.
//
// Lists documents matching the optional prefix. Returns one path per line
// for text output, or a JSON array for --json output.
func ExecLs(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	paths, err := sdk.Host.List(prefix)
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return sdk.RichResult{Text: "", Data: []string{}}, nil
	}

	return sdk.RichResult{
		Text: strings.Join(paths, "\n"),
		Data: paths,
	}, nil
}
