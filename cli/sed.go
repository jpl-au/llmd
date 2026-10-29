// sed.go provides sed-style substitution on documents.
//
// Usage:
//
//	llmd sed 's/old/new/' <path>...
//	llmd sed -i 's/old/new/' <path>    In-place (same as without -i)
//
// Only s/old/new/ substitution is supported. The delimiter after 's'
// can be any character (e.g. s|old|new|).

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var sedSpec = sdk.Command{
	Name: "sed", Desc: `Sed-style substitution on a document

Only the s (substitute) command is supported. The delimiter can be
any character, which is useful when replacing paths. By default the
search pattern must occur exactly once in the document; append the
trailing 'g' flag (e.g. s/old/new/g) to substitute every occurrence.
With several paths, the substitution is applied to each in turn.`, Usage: "sed [-i] 's/old/new/[g]' <path>...", MCP: true, NeedsAuthor: true,
	Flags: []sdk.Flag{
		{Name: "i", Type: "bool", Desc: "In place (always the case; accepted for familiarity)"},
		{Name: "message", Type: "string", Desc: "Version message"},
	},
}

func sed(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(sedSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}
	if len(positional) < 2 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "sed: an expression and a path are required.")
	}

	expr, paths := positional[0], positional[1:]
	old, new, global, err := parseSed(expr)
	if err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}

	lines := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := ctx.Documents.Edit(path, old, new, sdk.EditOpts{
			Author:     ctx.Author,
			Message:    flags.String("message"),
			ReplaceAll: global,
		}); err != nil {
			if errors.Is(err, sdk.ErrNotUnique) {
				return nil, sdk.Errorf(sdk.ErrNotUnique, "sed: %v Add surrounding text to make it unique, or end the expression with g to replace every one.", err)
			}
			return nil, fmt.Errorf("sed: %w", err)
		}
		lines = append(lines, "Edited "+path)
	}

	return sdk.Text(strings.Join(lines, "\n")), nil
}

// parseSed extracts old, new, and the global flag from a sed
// s-expression like "s/old/new/" or "s/old/new/g". The trailing 'g'
// flag opts into substituting every occurrence; without it, the search
// must match exactly once.
func parseSed(expr string) (string, string, bool, error) {
	if len(expr) < 4 || expr[0] != 's' {
		return "", "", false, sdk.Errorf(sdk.ErrInvalidArg, "%s is not a substitution. Write it as s/old/new/.", expr)
	}

	delim := expr[1]
	rest := expr[2:]

	idx := 0
	for idx < len(rest) && rest[idx] != delim {
		idx++
	}
	if idx >= len(rest) {
		return "", "", false, sdk.Errorf(sdk.ErrInvalidArg, "%s is not a substitution. Write it as s/old/new/.", expr)
	}

	old := rest[:idx]
	rest = rest[idx+1:]

	// Find closing delimiter; anything after it is trailing flags.
	idx = 0
	for idx < len(rest) && rest[idx] != delim {
		idx++
	}
	new := rest[:idx]

	var global bool
	if idx < len(rest) {
		for _, f := range rest[idx+1:] {
			if f == 'g' {
				global = true
			}
		}
	}

	if old == "" {
		return "", "", false, sdk.Errorf(sdk.ErrInvalidArg, "%s has an empty search pattern.", expr)
	}

	return old, new, global, nil
}
