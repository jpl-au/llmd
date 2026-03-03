package cli

// edit performs a search-and-replace within a document. Takes three
// positional args: path, old text, new text. The store replaces the
// first occurrence of old with new and creates a new version.
//
// This is the primary way MCP clients make targeted changes without
// rewriting the entire document via write.

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func edit(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("edit: %w", sdk.ErrMissingArg)
	}

	path, old, new := args[0], args[1], args[2]

	var message string
	for i := 3; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if after, ok := strings.CutPrefix(args[i], "--message="); ok {
			message = after
		}
	}

	if err := ctx.Documents.Edit(path, old, new, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}
