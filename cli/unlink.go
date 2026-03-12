// unlink.go removes links between documents.
//
// Usage:
//
//	llmd unlink <from> <to>

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var unlinkSpec = sdk.Command{
	Name: "unlink", Desc: `Remove a link between two documents

Both the source and target document paths must be specified.`, Usage: "unlink <from> <to>", MCP: true, NeedsAuthor: true,
}

func unlink(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("unlink: %w", sdk.ErrMissingArg)
	}

	from, to := args[0], args[1]
	if err := ctx.Links.Remove(from, to, ctx.Author); err != nil {
		return nil, fmt.Errorf("unlink: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Unlinked %s -> %s", from, to)), nil
}
