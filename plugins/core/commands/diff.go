//go:build wasip1

package commands

import (
	"fmt"
	"strconv"

	"github.com/jpl-au/llmd/sdk"
)

// Diff defines the diff command for comparing documents.
var Diff = sdk.Command{
	Name:        "diff",
	Description: "Compare documents or document versions",
	Usage:       "diff <source> [target]",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "C", Short: "C", Type: "int", Description: "Lines of context"},
		{Name: "stat", Type: "bool", Description: "Show stats only"},
	},
}

// ExecDiff executes the diff command.
//
// Usage:
//   - diff <path>                - compare previous version to latest
//   - diff <source> <target>     - compare source to target
//
// Source and target can be:
//   - Filesystem path (if file exists)
//   - llmd document path
//   - llmd path:version (e.g., notes/todo:3)
//   - 9-character document key
func ExecDiff(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	var paths []string
	var opts sdk.DiffOptions
	var statOnly bool

	// Parse args and flags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-C" && i+1 < len(args):
			i++
			opts.Context, _ = strconv.Atoi(args[i])
		case arg == "--stat":
			statOnly = true
		case arg[0] != '-':
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("diff: missing source argument")
	}

	var source, target string
	switch len(paths) {
	case 1:
		// Single argument: compare previous version to latest
		source, target = prevAndLatest(paths[0])
		if source == "" {
			return nil, fmt.Errorf("diff: no previous version for %s", paths[0])
		}
	case 2:
		source = paths[0]
		target = paths[1]
	default:
		return nil, fmt.Errorf("diff: expected 1 or 2 paths, got %d", len(paths))
	}

	result, err := sdk.Host.Diff(source, target, opts)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if statOnly {
		return sdk.TextResult(fmt.Sprintf("+%d -%d", result.Added, result.Removed)), nil
	}

	if result.Diff == "" {
		return sdk.TextResult("No differences"), nil
	}

	return sdk.TextResult(result.Diff), nil
}

// prevAndLatest returns path:prevVersion and path for diffing previous vs latest.
// Returns empty source if there's no previous version.
func prevAndLatest(path string) (source, target string) {
	versions, err := sdk.Host.History(path, 2)
	if err != nil || len(versions) < 2 {
		return "", ""
	}
	// versions[0] is latest, versions[1] is previous
	return path + ":" + strconv.Itoa(versions[1].Version), path
}
