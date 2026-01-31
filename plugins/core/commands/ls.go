//go:build wasip1

// This file implements the ls command for listing documents.
package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// Ls defines the ls command for listing documents.
//
// The ls command lists documents in the store, similar to the Unix ls command.
// It supports filtering by prefix and various display formats.
var Ls = sdk.Command{
	Name:        "ls",
	Description: "List documents",
	Usage:       "ls [prefix]",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "l", Type: "bool", Description: "Long format with details"},
		{Name: "a", Type: "bool", Description: "Include deleted documents"},
		{Name: "r", Type: "bool", Description: "Reverse sort order"},
		{Name: "t", Type: "bool", Description: "Sort by time (newest first)"},
	},
}

// ExecLs executes the ls command.
//
// Lists documents matching the optional prefix. Returns one path per line
// for text output, or a JSON array for --json output.
func ExecLs(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	// Parse args: flags and prefix
	var long, all, reverse, sortByTime bool
	var prefix string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			// Parse combined short flags like -la
			for _, c := range arg[1:] {
				switch c {
				case 'l':
					long = true
				case 'a':
					all = true
				case 'r':
					reverse = true
				case 't':
					sortByTime = true
				}
			}
		} else if !strings.HasPrefix(arg, "-") {
			prefix = arg
		}
	}

	sort := "path"
	if sortByTime {
		sort = "time"
	}

	docs, err := sdk.Host.List(prefix, sdk.ListOptions{
		IncludeDeleted: all,
		Sort:           sort,
		Reverse:        reverse,
	})
	if err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return sdk.RichResult{Text: "", Data: []sdk.Document{}}, nil
	}

	// Build structured data (always full documents for --json)
	data := make([]map[string]any, len(docs))
	for i, d := range docs {
		data[i] = map[string]any{
			"path":       d.Path,
			"version":    d.Version,
			"author":     d.Author,
			"message":    d.Message,
			"created_at": d.CreatedAt,
			"deleted":    d.Deleted,
		}
	}

	// Build text output
	var text string
	if long {
		text = formatTable(docs)
	} else {
		paths := make([]string, len(docs))
		for i, d := range docs {
			paths[i] = d.Path
		}
		text = strings.Join(paths, "\n")
	}

	return sdk.RichResult{Text: text, Data: data}, nil
}

// formatTable formats documents as a table with headers.
func formatTable(docs []sdk.Document) string {
	if len(docs) == 0 {
		return ""
	}

	// Calculate column widths
	verWidth := 3 // "VER"
	authorWidth := 6 // "AUTHOR"
	for _, d := range docs {
		if w := len(fmt.Sprintf("%d", d.Version)); w > verWidth {
			verWidth = w
		}
		if w := len(d.Author); w > authorWidth {
			authorWidth = w
		}
	}

	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "%*s  %-*s  %-10s  %s\n", verWidth, "VER", authorWidth, "AUTHOR", "DATE", "PATH")

	// Rows
	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		path := d.Path
		if d.Deleted {
			path = d.Path + " (deleted)"
		}
		fmt.Fprintf(&b, "%*d  %-*s  %-10s  %s\n", verWidth, d.Version, authorWidth, d.Author, date, path)
	}

	return strings.TrimSuffix(b.String(), "\n")
}
