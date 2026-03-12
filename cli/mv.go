package cli

// mv renames or moves a document to a new path. The full version
// history moves with it.

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var mvSpec = sdk.Command{
	Name: "mv", Desc: `Move or rename a document, preserving history

The full version history moves with the document. The source path
must exist and the destination path must not.`, Usage: "mv <from> <to>", MCP: true, NeedsAuthor: true,
}

func mv(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("mv: %w", sdk.ErrMissingArg)
	}

	if err := ctx.Documents.Move(args[0], args[1], ctx.Author); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved %s -> %s", args[0], args[1])), nil
}
