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

Replaces <old> with <new>, creating a new version with the change
applied. By default <old> must occur exactly once in the document - if
it appears multiple times, expand the search string with surrounding
context to disambiguate, or pass --all to substitute every occurrence.
Use sed for pattern-based substitution instead of literal strings.`, Usage: "edit [--all] [--message <msg>] <path> <old> <new>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
		{Name: "message", Type: "string", Desc: "Version message"},
		{Name: "all", Type: "bool", Desc: "Replace all occurrences instead of requiring a unique match"},
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

	if err := ctx.Documents.Edit(path, old, new, sdk.EditOpts{
		Author:     ctx.Author,
		Message:    flags.String("message"),
		ReplaceAll: flags.Bool("all"),
	}); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}
