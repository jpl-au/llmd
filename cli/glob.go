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
Output is one path per line.`, Usage: "glob <pattern>", MCP: true, MCPName: "llmd_glob",
}

func glob(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("glob: %w", sdk.ErrMissingArg)
	}

	paths, err := ctx.Documents.Glob(args[0])
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(paths) == 0 {
		return sdk.Result{Text: "", Data: []string{}}, nil
	}

	return sdk.Result{Text: strings.Join(paths, "\n"), Data: paths}, nil
}
