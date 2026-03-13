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

var linkSpec = sdk.Command{
	Name: "link", Desc: `Create or list links between documents

With two paths, creates a directional link from source to target.
With one path, lists outgoing links from that document. Use --in
to see incoming links instead.`, Usage: "link [options] <from> [to]", MCP: true, Flags: []sdk.Flag{
		{Name: "label", Type: "string", Desc: "Link label"},
		{Name: "in", Type: "bool", Desc: "Show incoming links"},
	},
}

func linkCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(linkSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}
	label := flags.String("label")
	var dir string
	if flags.Bool("in") {
		dir = "in"
	}

	if len(positional) == 0 {
		return nil, fmt.Errorf("link: %w", sdk.ErrMissingArg)
	}

	// llmd link <path> — list links
	if len(positional) == 1 {
		ll, err := ctx.Links.List(positional[0], dir)
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

	// Checked here, not via NeedsAuthor — reads don't need an author.
	if ctx.Author == "" {
		return nil, fmt.Errorf("link: author required for mutations")
	}
	from, to := positional[0], positional[1]
	if err := ctx.Links.Add(from, to, label, ctx.Author); err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Linked %s -> %s", from, to)), nil
}
