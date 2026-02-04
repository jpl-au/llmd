package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func restore(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("restore: missing path argument")
	}

	if err := sdk.API.Restore(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored %s", args[0])), nil
}
