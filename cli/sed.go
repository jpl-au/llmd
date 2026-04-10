// sed.go provides sed-style substitution on documents.
//
// Usage:
//
//	llmd sed 's/old/new/' <path>
//	llmd sed -i 's/old/new/' <path>    In-place (same as without -i)
//
// Only s/old/new/ substitution is supported. The delimiter after 's'
// can be any character (e.g. s|old|new|).

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var sedSpec = sdk.Command{
	Name: "sed", Desc: `Sed-style substitution on a document

Only the s (substitute) command is supported. The delimiter can be
any character, which is useful when replacing paths. By default the
search pattern must occur exactly once in the document; append the
trailing 'g' flag (e.g. s/old/new/g) to substitute every occurrence.`, Usage: "sed [-i] 's/old/new/[g]' <path>", MCP: true, NeedsAuthor: true,
}

func sed(ctx sdk.Context, args []string) (sdk.Response, error) {
	var expr, path string

	for _, arg := range args {
		if arg == "-i" {
			continue // -i is default behavior (always in-place)
		}
		if expr == "" {
			expr = arg
		} else {
			path = arg
		}
	}

	if expr == "" || path == "" {
		return nil, fmt.Errorf("sed: %w", sdk.ErrMissingArg)
	}

	old, new, global, err := parseSed(expr)
	if err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}

	if err := ctx.Documents.Edit(path, old, new, sdk.EditOpts{
		Author:     ctx.Author,
		ReplaceAll: global,
	}); err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}

// parseSed extracts old, new, and the global flag from a sed
// s-expression like "s/old/new/" or "s/old/new/g". The trailing 'g'
// flag opts into substituting every occurrence; without it, the search
// must match exactly once.
func parseSed(expr string) (string, string, bool, error) {
	if len(expr) < 4 || expr[0] != 's' {
		return "", "", false, fmt.Errorf("%w: expected s/old/new/", sdk.ErrInvalidArg)
	}

	delim := expr[1]
	rest := expr[2:]

	idx := 0
	for idx < len(rest) && rest[idx] != delim {
		idx++
	}
	if idx >= len(rest) {
		return "", "", false, fmt.Errorf("%w: missing delimiter in sed expression", sdk.ErrInvalidArg)
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
		return "", "", false, fmt.Errorf("%w: empty search pattern", sdk.ErrInvalidArg)
	}

	return old, new, global, nil
}
