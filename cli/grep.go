package cli

// grep searches document content using FTS5 full-text search.
//
// It supports three output modes, matching standard grep conventions:
//   - Default: prints "path:text" per match (or "path:line:text" with -n)
//   - -l (files only): prints unique paths, deduplicating multiple hits per doc
//   - -c (count only): prints "path:count" per document
//
// The first positional arg is the search pattern; the optional second
// limits results to a path prefix (e.g. "grep TODO notes/").
//
// -C accepts both "-C3" and "-C 3" forms to match GNU grep conventions.

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var grepSpec = sdk.Command{
	Name: "grep", Desc: `Search document content using full-text search

Searches all documents (or those under <path>) for the given query
using SQLite FTS5 syntax: AND (implicit), OR, NOT, NEAR(), prefix
*, and phrases "...".`, Usage: "grep [options] <pattern> [path]", MCP: true, MCPName: "llmd_grep", Flags: []sdk.Flag{
		{Name: "n", Type: "bool", Desc: "Show line numbers"},
		{Name: "l", Type: "bool", Desc: "Show only filenames"},
		{Name: "c", Type: "bool", Desc: "Show match count only"},
		{Name: "C", Type: "int", Desc: "Lines of context"},
	},
}

func grep(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(grepSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}
	showLineNums := flags.Bool("n")
	filesOnly := flags.Bool("l")
	countOnly := flags.Bool("c")
	contextLines := flags.Int("C")

	var pattern, pathPrefix string
	if len(positional) > 0 {
		pattern = positional[0]
	}
	if len(positional) > 1 {
		pathPrefix = positional[1]
	}

	if pattern == "" {
		return nil, fmt.Errorf("grep: %w", sdk.ErrMissingArg)
	}

	results, err := ctx.Documents.Grep(pattern, sdk.GrepOpts{Path: pathPrefix, Context: contextLines})
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.Result{Text: "", Data: []sdk.GrepHit{}}, nil
	}

	var text string
	if countOnly {
		counts := make(map[string]int)
		for _, r := range results {
			counts[r.Path]++
		}
		var out strings.Builder
		for path, count := range counts {
			fmt.Fprintf(&out, "%s:%d\n", path, count)
		}
		text = strings.TrimSuffix(out.String(), "\n")
	} else if filesOnly {
		// Deduplicate paths - a document may have multiple hits.
		seen := make(map[string]bool)
		var paths []string
		for _, r := range results {
			if !seen[r.Path] {
				seen[r.Path] = true
				paths = append(paths, r.Path)
			}
		}
		text = strings.Join(paths, "\n")
	} else {
		var out strings.Builder
		for _, r := range results {
			if showLineNums {
				fmt.Fprintf(&out, "%s:%d:%s\n", r.Path, r.Line, r.Text)
			} else {
				fmt.Fprintf(&out, "%s:%s\n", r.Path, r.Text)
			}
		}
		text = strings.TrimSuffix(out.String(), "\n")
	}

	return sdk.Result{Text: text, Data: results}, nil
}
