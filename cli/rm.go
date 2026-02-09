package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func rm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rm: missing path argument")
	}

	if err := sdk.API.Delete(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Deleted %s", args[0])), nil
}
