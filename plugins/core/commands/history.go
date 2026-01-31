//go:build wasip1

package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// History defines the history command for viewing document versions.
var History = sdk.Command{
	Name:        "history",
	Description: "Show version history for a document",
	Usage:       "history [-n limit] <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "n", Type: "int", Description: "Maximum versions to show"},
	},
}

// ExecHistory executes the history command.
func ExecHistory(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	// Parse args
	var path string
	var limit int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" && i+1 < len(args) {
			i++
			limit, _ = strconv.Atoi(args[i])
		} else if strings.HasPrefix(arg, "-n") {
			limit, _ = strconv.Atoi(arg[2:])
		} else if !strings.HasPrefix(arg, "-") {
			path = arg
		}
	}

	if path == "" {
		return nil, fmt.Errorf("history: missing path argument")
	}

	versions, err := sdk.Host.History(path, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if len(versions) == 0 {
		return sdk.RichResult{Text: "No history found", Data: []sdk.VersionInfo{}}, nil
	}

	// Build text output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %-12s %-20s %s\n", "Version", "Author", "Date", "Message"))
	sb.WriteString(strings.Repeat("-", 60) + "\n")

	for _, v := range versions {
		date := time.UnixMilli(v.CreatedAt).Format("2006-01-02 15:04:05")
		msg := v.Message
		if msg == "" {
			msg = "-"
		}
		sb.WriteString(fmt.Sprintf("%-8d %-12s %-20s %s\n", v.Version, truncate(v.Author, 12), date, msg))
	}

	return sdk.RichResult{
		Text: strings.TrimSuffix(sb.String(), "\n"),
		Data: versions,
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
