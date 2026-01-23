//go:build wasip1

package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Diff defines the diff command for comparing document versions.
var Diff = sdk.Command{
	Name:        "diff",
	Description: "Compare document versions",
	Usage:       "diff <path> [version1] [version2]",
	MCPEnabled:  true,
	Flags:       []sdk.Flag{},
}

// ExecDiff executes the diff command.
//
// Usage:
//   - diff <path>           - diff previous vs latest
//   - diff <path> <v>       - diff version v vs latest
//   - diff <path> <v1> <v2> - diff v1 vs v2
func ExecDiff(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("diff: missing path argument")
	}

	path := args[0]
	v1, v2 := 0, 0

	switch len(args) {
	case 1:
		// diff previous vs latest - let host figure it out
		v1, v2 = 0, 0
	case 2:
		// diff specific version vs latest
		var err error
		v1, err = parseVersion(args[1])
		if err != nil {
			return nil, fmt.Errorf("diff: invalid version: %s", args[1])
		}
	case 3:
		// diff v1 vs v2
		var err error
		v1, err = parseVersion(args[1])
		if err != nil {
			return nil, fmt.Errorf("diff: invalid version1: %s", args[1])
		}
		v2, err = parseVersion(args[2])
		if err != nil {
			return nil, fmt.Errorf("diff: invalid version2: %s", args[2])
		}
	}

	result, err := sdk.Host.Diff(path, v1, v2)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if result == "" {
		return sdk.TextResult("No differences"), nil
	}

	return sdk.TextResult(result), nil
}

func parseVersion(s string) (int, error) {
	s = strings.TrimPrefix(s, "v")
	return strconv.Atoi(s)
}
