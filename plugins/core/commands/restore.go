//go:build wasip1

package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Restore defines the restore command for restoring deleted documents.
var Restore = sdk.Command{
	Name:        "restore",
	Description: "Restore a deleted document",
	Usage:       "restore <path>",
	MCPEnabled:  true,
	Flags:       []sdk.Flag{},
}

// ExecRestore executes the restore command.
func ExecRestore(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("restore: missing path argument")
	}

	path := args[0]

	if err := sdk.Host.Restore(path, ctx.Author); err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Restored %s", path)), nil
}
