package cli

import (
	"fmt"
	"strconv"

	"github.com/jpl-au/llmd/sdk"
)

func diffCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var paths []string
	var contextLines int
	var statOnly bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case arg == "--stat":
			statOnly = true
		case len(arg) > 0 && arg[0] != '-':
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("diff: missing source argument")
	}

	var source, target string
	switch len(paths) {
	case 1:
		versions, err := sdk.API.History(paths[0], 2)
		if err != nil || len(versions) < 2 {
			return nil, fmt.Errorf("diff: no previous version for %s", paths[0])
		}
		source = paths[0] + ":" + strconv.Itoa(versions[1].Num)
		target = paths[0]
	case 2:
		source, target = paths[0], paths[1]
	default:
		return nil, fmt.Errorf("diff: expected 1 or 2 paths, got %d", len(paths))
	}

	diffText, added, removed, err := sdk.API.Diff(source, target, contextLines)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if statOnly {
		return sdk.Text(fmt.Sprintf("+%d -%d", added, removed)), nil
	}

	if diffText == "" {
		return sdk.Text("No differences"), nil
	}

	return sdk.Text(diffText), nil
}
