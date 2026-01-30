//go:build wasip1

// This file implements the glob command for finding documents by path pattern.
package commands

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Glob defines the glob command for finding documents by path pattern.
//
// The glob command finds documents whose paths match a glob pattern.
// Supports *, **, and ? wildcards.
var Glob = sdk.Command{
	Name:        "glob",
	Description: "Find documents by path pattern",
	Usage:       "glob <pattern>",
	MCPEnabled:  true,
	MCPName:     "llmd_glob",
	Flags:       []sdk.Flag{},
}

// ExecGlob executes the glob command.
//
// Finds documents matching the glob pattern. Returns one path per line
// for text output, or a JSON array for --json output.
func ExecGlob(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("glob: missing pattern argument")
	}

	pattern := args[0]

	paths, err := sdk.Host.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(paths) == 0 {
		return sdk.RichResult{Text: "", Data: []string{}}, nil
	}

	return sdk.RichResult{
		Text: strings.Join(paths, "\n"),
		Data: paths,
	}, nil
}
