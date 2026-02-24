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

	old, new, err := parseSed(expr)
	if err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}

	if err := sdk.Documents.Edit(path, old, new, ctx.Author, ""); err != nil {
		return nil, fmt.Errorf("sed: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}

// parseSed extracts old and new from a sed s-expression like "s/old/new/".
func parseSed(expr string) (string, string, error) {
	if len(expr) < 4 || expr[0] != 's' {
		return "", "", fmt.Errorf("%w: expected s/old/new/", sdk.ErrInvalidArg)
	}

	delim := expr[1]
	rest := expr[2:]

	idx := 0
	for idx < len(rest) && rest[idx] != delim {
		idx++
	}
	if idx >= len(rest) {
		return "", "", fmt.Errorf("%w: missing delimiter in sed expression", sdk.ErrInvalidArg)
	}

	old := rest[:idx]
	rest = rest[idx+1:]

	// Find closing delimiter (optional trailing flags like 'g' ignored)
	idx = 0
	for idx < len(rest) && rest[idx] != delim {
		idx++
	}
	new := rest[:idx]

	if old == "" {
		return "", "", fmt.Errorf("%w: empty search pattern", sdk.ErrInvalidArg)
	}

	return old, new, nil
}
