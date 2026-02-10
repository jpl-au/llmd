// link.go creates directed links between documents.
//
// Usage:
//
//	llmd link <from> <to>                Create a link
//	llmd link --label <label> <from> <to>  Create a labeled link
//	llmd link <path>                     List outgoing links
//	llmd link --in <path>                List incoming links

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func linkCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var label, dir string
	var positional []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--label" && i+1 < len(args):
			i++
			label = args[i]
		case strings.HasPrefix(args[i], "--label="):
			label = strings.TrimPrefix(args[i], "--label=")
		case args[i] == "--in":
			dir = "in"
		case args[i] == "--both":
			dir = "both"
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return nil, fmt.Errorf("link: %w", sdk.ErrMissingArg)
	}

	// llmd link <path> — list links
	if len(positional) == 1 {
		ll, err := sdk.API.LinkList(positional[0], dir)
		if err != nil {
			return nil, fmt.Errorf("link: %w", err)
		}
		var lines []string
		for _, l := range ll {
			if l.Label != "" {
				lines = append(lines, fmt.Sprintf("%s -> %s (%s)", l.From, l.To, l.Label))
			} else {
				lines = append(lines, fmt.Sprintf("%s -> %s", l.From, l.To))
			}
		}
		return sdk.Result{Text: strings.Join(lines, "\n"), Data: ll}, nil
	}

	// llmd link <from> <to> — create link
	from, to := positional[0], positional[1]
	if err := sdk.API.LinkAdd(from, to, label, ctx.Author); err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Linked %s -> %s", from, to)), nil
}
