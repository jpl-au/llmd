// find.go performs full-text search and returns matching document paths.
// Unlike grep (which returns content), find returns only paths — similar
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

func find(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("find: %w", sdk.ErrMissingArg)
	}

	query := args[0]
	var pathPrefix string
	if len(args) > 1 {
		pathPrefix = args[1]
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

	return sdk.Result{Text: strings.Join(paths, "\n"), Data: paths}, nil
}
