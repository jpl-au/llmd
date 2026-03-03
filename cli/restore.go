package cli

// restore recovers a soft-deleted document, making it visible again
// in ls output. Only works on documents deleted with "rm" that haven't
// been purged by "vacuum".

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

func restore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("restore: %w", sdk.ErrMissingArg)
	}

	if err := ctx.Documents.Restore(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored %s", args[0])), nil
}
