package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

func historyCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
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

	versions, err := sdk.API.History(path, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if len(versions) == 0 {
		return sdk.Result{Text: "No history found", Data: []sdk.Version{}}, nil
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
		author := v.Author
		if len(author) > 12 {
			author = author[:11] + "…"
		}
		sb.WriteString(fmt.Sprintf("%-8d %-12s %-20s %s\n", v.Num, author, date, msg))
	}

	return sdk.Result{Text: strings.TrimSuffix(sb.String(), "\n"), Data: versions}, nil
}
