// find.go performs full-text search and returns matching document paths.
// Unlike grep (which returns content), find returns only paths - similar
// to grep -l but using FTS5 full-text search.
//
// Usage:
//
//	llmd find <query>             Search all documents
//	llmd find <query> <prefix>    Search under a path prefix

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var findSpec = sdk.Command{
	Name: "find", Desc: `Full-text search returning matching paths only

Like grep, but prints only document paths - no content. Useful for
piping or getting a quick overview of which documents match. Defaults
to 500 matches; use --limit or --all to change the cap.`, Usage: "find [flags] <query> [path]", MCP: true, MCPName: "llmd_find", Flags: []sdk.Flag{
		{Name: "limit", Short: "n", Type: "int", Desc: "Maximum paths to return (default 500)"},
		{Name: "all", Type: "bool", Desc: "Return every match, no limit"},
	},
}

func find(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(findSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("find: %w", err)
	}
	if len(positional) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "find: a search query is required.")
	}

	query := positional[0]
	var pathPrefix string
	if len(positional) > 1 {
		pathPrefix = positional[1]
	}

	results, err := ctx.Documents.Grep(query, sdk.GrepOpts{
		Path: pathPrefix,
		Mode: sdk.GrepPaths,
	})
	if err != nil {
		return nil, fmt.Errorf("find: %w", err)
	}

	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}

	// Apply the cap: --all beats --limit beats the default.
	paths, hidden := capped(paths, flags)

	return sdk.Result{Text: strings.Join(paths, "\n"), Data: paths, Note: cappedNote(hidden)}, nil
}
