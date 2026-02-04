package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func mv(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mv: requires <from> <to> arguments")
	}

	if err := sdk.API.Move(args[0], args[1], ctx.Author); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved %s -> %s", args[0], args[1])), nil
}
