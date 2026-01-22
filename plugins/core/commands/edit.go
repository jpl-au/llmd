//go:build wasip1

package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Edit defines the edit command for search/replace operations.
var Edit = sdk.Command{
	Name:        "edit",
	Description: "Edit a document via search/replace",
	Usage:       "edit <path> <old> <new>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "message", Short: "m", Type: "string", Description: "Version message"},
	},
}

// ExecEdit executes the edit command.
//
// Performs a search/replace on a document. The old text must exist in the
// document exactly once (unless --all is specified). Creates a new version
// with the replacement applied.
func ExecEdit(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("edit: requires <path> <old> <new> arguments")
	}

	path := args[0]
	old := args[1]
	new := args[2]

	message, _ := flags["message"].(string)

	if err := sdk.Host.Edit(path, old, new, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Edited %s", path)), nil
}
