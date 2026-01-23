//go:build wasip1

package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// History defines the history command for viewing document versions.
var History = sdk.Command{
	Name:        "history",
	Description: "Show version history for a document",
	Usage:       "history <path>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "limit", Short: "n", Type: "int", Description: "Maximum versions to show"},
	},
}

// ExecHistory executes the history command.
func ExecHistory(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("history: missing path argument")
	}

	path := args[0]

	limit := 0
	if v, ok := flags["limit"].(int); ok {
		limit = v
	}

	versions, err := sdk.Host.History(path, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if len(versions) == 0 {
		return sdk.TextResult("No history found"), nil
	}

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

	return sdk.TextResult(sb.String()), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
