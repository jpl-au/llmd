//go:build wasip1

// This file implements the grep command for searching documents.
package commands

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Grep defines the grep command for searching documents.
//
// The grep command searches document content using FTS5 full-text search.
// It returns matching documents with their path and content.
//
// MCPName is set to "llmd_grep" to avoid conflicts with existing grep tools
// that AI assistants may have access to.
var Grep = sdk.Command{
	Name:        "grep",
	Description: "Search documents with full-text search",
	Usage:       "grep <query> [path]",
	MCPEnabled:  true,
	MCPName:     "llmd_grep",
	Flags:       []sdk.Flag{},
}

// ExecGrep executes the grep command.
//
// Searches all documents for content matching the FTS5 query. Results are
// formatted as "path:content". Returns an empty string if no matches are found.
func ExecGrep(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("grep: missing query argument")
	}

	query := args[0]

	results, err := sdk.Host.Grep(query)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.TextResult(""), nil
	}

	var out strings.Builder
	for _, r := range results {
		// Truncate content for display
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		// Replace newlines for single-line output
		content = strings.ReplaceAll(content, "\n", " ")
		out.WriteString(fmt.Sprintf("%s: %s\n", r.Path, content))
	}

	return sdk.TextResult(out.String()), nil
}
