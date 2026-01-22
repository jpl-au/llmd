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
// The grep command searches document content using regular expressions,
// similar to the Unix grep command. It supports case-insensitive search,
// context lines, and match counting.
//
// MCPName is set to "llmd_grep" to avoid conflicts with existing grep tools
// that AI assistants may have access to.
var Grep = sdk.Command{
	Name:        "grep",
	Description: "Search documents with regex",
	Usage:       "grep <pattern> [path]",
	MCPEnabled:  true,
	MCPName:     "llmd_grep",
	Flags: []sdk.Flag{
		{Name: "context", Short: "C", Type: "int", Description: "Lines of context"},
		{Name: "ignore-case", Short: "i", Type: "bool", Description: "Case insensitive"},
		{Name: "invert", Short: "v", Type: "bool", Description: "Invert match"},
		{Name: "count", Short: "c", Type: "bool", Description: "Count matches only"},
	},
}

// ExecGrep executes the grep command.
//
// Searches all documents for lines matching the regex pattern. Results are
// formatted as "path:line:content" similar to Unix grep. Returns an empty
// string if no matches are found.
func ExecGrep(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("grep: missing pattern argument")
	}

	pattern := args[0]

	results, err := sdk.Host.Grep(pattern)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.TextResult(""), nil
	}

	var out strings.Builder
	for _, r := range results {
		out.WriteString(fmt.Sprintf("%s:%d:%s\n", r.Path, r.Line, r.Content))
	}

	return sdk.TextResult(out.String()), nil
}
