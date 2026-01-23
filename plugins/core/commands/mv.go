//go:build wasip1

package commands

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// Mv defines the mv command for moving/renaming documents.
var Mv = sdk.Command{
	Name:        "mv",
	Description: "Move or rename a document",
	Usage:       "mv <from> <to>",
	MCPEnabled:  true,
	Flags:       []sdk.Flag{},
}

// ExecMv executes the mv command.
func ExecMv(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mv: requires <from> <to> arguments")
	}

	from := args[0]
	to := args[1]

	if err := sdk.Host.Move(from, to, ctx.Author); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Moved %s -> %s", from, to)), nil
}
