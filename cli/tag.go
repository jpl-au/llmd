// tag.go manages document tags.
//
// Usage:
//
//	llmd tag <path> <name>           Add a tag
//	llmd tag -d <path> <name>        Remove a tag
//	llmd tag <path>                  List tags on a document
//	llmd tag                         List all tags with counts
//	llmd tag -f <name>               Find documents with a tag

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func tag(ctx sdk.Context, args []string) (sdk.Response, error) {
	var remove, find bool

	var positional []string
	for i := range args {
		switch args[i] {
		case "-d":
			remove = true
		case "-f":
			find = true
		default:
			positional = append(positional, args[i])
		}
	}

	// llmd tag -f <name> — find documents with tag
	if find {
		if len(positional) == 0 {
			return nil, fmt.Errorf("tag: %w", sdk.ErrMissingArg)
		}
		paths, err := ctx.Tags.Find(positional[0])
		if err != nil {
			return nil, fmt.Errorf("tag: %w", err)
		}
		return sdk.Result{Text: strings.Join(paths, "\n"), Data: paths}, nil
	}

	// llmd tag — list all tags
	if len(positional) == 0 {
		infos, err := ctx.Tags.All()
		if err != nil {
			return nil, fmt.Errorf("tag: %w", err)
		}
		var lines []string
		for _, info := range infos {
			lines = append(lines, fmt.Sprintf("%s (%d)", info.Name, info.Count))
		}
		return sdk.Result{Text: strings.Join(lines, "\n"), Data: infos}, nil
	}

	// llmd tag <path> — list tags on a document
	if len(positional) == 1 {
		tags, err := ctx.Tags.List(positional[0])
		if err != nil {
			return nil, fmt.Errorf("tag: %w", err)
		}
		var names []string
		for _, t := range tags {
			names = append(names, t.Name)
		}
		return sdk.Result{Text: strings.Join(names, "\n"), Data: tags}, nil
	}

	path, name := positional[0], positional[1]

	// llmd tag -d <path> <name> — remove
	if remove {
		if err := ctx.Tags.Remove(path, name, ctx.Author); err != nil {
			return nil, fmt.Errorf("tag: %w", err)
		}
		return sdk.Text(fmt.Sprintf("Removed tag %s from %s", name, path)), nil
	}

	// llmd tag <path> <name> — add
	if err := ctx.Tags.Add(path, name, ctx.Author); err != nil {
		return nil, fmt.Errorf("tag: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Tagged %s with %s", path, name)), nil
}
