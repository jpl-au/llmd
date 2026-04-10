package cli

// rm soft-deletes a document. The content is preserved and can be
// recovered with "restore". Use "vacuum" to permanently purge
// soft-deleted documents.

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var rmSpec = sdk.Command{
	Name: "rm", Desc: `Soft-delete a document (recoverable with restore)

The document is hidden from ls but its content and history are
preserved. Use restore to bring it back, or vacuum to permanently
purge all deleted documents.`, Usage: "rm <path>", MCP: true, NeedsAuthor: true,
}

func rm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rm: %w", sdk.ErrMissingArg)
	}

	if err := ctx.Documents.Delete(args[0], sdk.DeleteOpts{Author: ctx.Author}); err != nil {
		return nil, fmt.Errorf("rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Deleted %s", args[0])), nil
}
