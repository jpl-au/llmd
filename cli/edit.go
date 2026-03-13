package cli

// edit performs a search-and-replace within a document. Takes three
// positional args: path, old text, new text. The store replaces the
// first occurrence of old with new and creates a new version.
//
// This is the primary way MCP clients make targeted changes without
// rewriting the entire document via write.

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var editSpec = sdk.Command{
	Name: "edit", Desc: `Search and replace text within a document

Replaces the first occurrence of <old> with <new>, creating a new
version with the change applied. Use sed for pattern-based
substitution instead of literal strings.`, Usage: "edit <path> <old> <new>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
		{Name: "message", Type: "string", Desc: "Version message"},
	},
}

func edit(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(editSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}
	if len(positional) < 3 {
		return nil, fmt.Errorf("edit: %w", sdk.ErrMissingArg)
	}

	path, old, new := positional[0], positional[1], positional[2]
	message := flags.String("message")

	if err := ctx.Documents.Edit(path, old, new, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}
