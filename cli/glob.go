package cli

// glob finds documents whose paths match a glob pattern.
// Patterns use standard glob syntax: * matches any sequence within a
// path segment, ** matches across segments.

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var globSpec = sdk.Command{
	Name: "glob", Desc: `Find documents by shell-style path pattern

Supports * (single level), ** (recursive), and ? (one character).
Output is one path per line. Defaults to 500 matches; use --limit
or --all to change the cap.`, Usage: "glob [flags] <pattern>", MCP: true, MCPName: "llmd_glob", Flags: []sdk.Flag{
		{Name: "limit", Type: "int", Desc: "Maximum paths to return (default 500)"},
		{Name: "all", Type: "bool", Desc: "Return every match, no limit"},
	},
}

func glob(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(globSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("glob: %w", sdk.ErrMissingArg)
	}

	paths, err := ctx.Documents.Glob(positional[0])
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	// Apply the cap: --all beats --limit beats the default.
	limit := flags.Int("limit")
	if flags.Bool("all") {
		limit = 0
	} else if limit == 0 {
		limit = defaultListLimit
	}
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}

	if len(paths) == 0 {
		return sdk.Result{Text: "", Data: []string{}}, nil
	}

	return sdk.Result{Text: strings.Join(paths, "\n"), Data: paths}, nil
}
