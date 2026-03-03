package cli

// mv renames or moves a document to a new path. The full version
// history moves with it.

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func mv(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mv: %w", sdk.ErrMissingArg)
	}

	if err := ctx.Documents.Move(args[0], args[1], ctx.Author); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved %s -> %s", args[0], args[1])), nil
}
