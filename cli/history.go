package cli

// history shows the version log for a document.
//
// Output is a lipgloss table: version number, author, date, and
// commit message.
//
// Limit defaults to 0 (all versions). Use -n to cap the output, e.g.
// "history -n5 notes/readme" shows the 5 most recent versions.

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
		return nil, fmt.Errorf("history: %w", sdk.ErrMissingArg)
	}

	versions, err := sdk.API.History(path, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if len(versions) == 0 {
		return sdk.Result{Text: "No history found", Data: []sdk.Version{}}, nil
	}

	t := newTable("VER", "AUTHOR", "DATE", "MESSAGE")

	for _, v := range versions {
		date := time.UnixMilli(v.CreatedAt).Format("2006-01-02 15:04")
		msg := v.Message
		if msg == "" {
			msg = "-"
		}
		t.Row(fmt.Sprintf("%d", v.Num), v.Author, date, msg)
	}

	return sdk.Result{Text: t.String(), Data: versions}, nil
}
