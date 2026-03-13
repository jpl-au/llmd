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
	"time"

	"github.com/jpl-au/llmd/sdk"
)

var historySpec = sdk.Command{
	Name: "history", Desc: `Show the version history of a document

Displays a table of all versions with version number, author, date,
and commit message. Newest versions are shown first.`, Usage: "history [-n limit] <path>", MCP: true, Flags: []sdk.Flag{
		{Name: "n", Type: "int", Desc: "Maximum versions to show"},
	},
}

func historyCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(historySpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	limit := flags.Int("n")

	var path string
	if len(positional) > 0 {
		path = positional[0]
	}
	if path == "" {
		return nil, fmt.Errorf("history: %w", sdk.ErrMissingArg)
	}

	versions, err := ctx.Documents.History(path, limit)
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
		t.Row(fmt.Sprintf("%d", v.Number), v.Author, date, msg)
	}

	return sdk.Result{Text: t.String(), Data: versions}, nil
}
