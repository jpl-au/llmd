package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func grep(ctx sdk.Context, args []string) (sdk.Result, error) {
	var pattern, pathPrefix string
	var showLineNums, filesOnly, countOnly bool
	var contextLines int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n":
			showLineNums = true
		case arg == "-l":
			filesOnly = true
		case arg == "-c":
			countOnly = true
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(arg, "-C"):
			contextLines, _ = strconv.Atoi(arg[2:])
		case !strings.HasPrefix(arg, "-"):
			if pattern == "" {
				pattern = arg
			} else {
				pathPrefix = arg
			}
		}
	}

	if pattern == "" {
		return nil, fmt.Errorf("grep: missing pattern argument")
	}

	results, err := sdk.API.Grep(pattern, sdk.GrepOpts{Path: pathPrefix, Context: contextLines})
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.Rich{Text: "", Data: []sdk.GrepHit{}}, nil
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

	return sdk.Rich{Text: text, Data: results}, nil
}
