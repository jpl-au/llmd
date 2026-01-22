//go:build wasip1

package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Rm defines the rm command for deleting documents.
var Rm = sdk.Command{
	Name:        "rm",
	Description: "Delete a document",
	Usage:       "rm <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "message", Short: "m", Type: "string", Description: "Deletion message"},
	},
}

// ExecRm executes the rm command.
//
// Soft-deletes a document from the store. The document can be restored
// later with the restore command. Permanent deletion only happens via vacuum.
func ExecRm(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rm: missing path argument")
	}

	path := args[0]

	if err := sdk.Host.Delete(path, ctx.Author); err != nil {
		return nil, fmt.Errorf("rm: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Deleted %s", path)), nil
}
