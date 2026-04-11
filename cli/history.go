package cli

// history shows the version log for a document.
//
// Output is a lipgloss table: version number, author, date, and
// commit message.
//
// The default limit is 10 versions so an agent running history on a
// heavily-edited document doesn't dump hundreds of rows into its
// context. Pass -n explicitly to cap at a different number, or
// --all for the full history.

import (
	"fmt"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// defaultHistoryLimit caps history output for the common case. Agents
// asking for "the history of this doc" almost always want the last
// few versions, not the whole log. 10 fits in a terminal and is
// sufficient for the typical review workflow. --all overrides.
const defaultHistoryLimit = 10

var historySpec = sdk.Command{
	Name: "history", Desc: `Show the version history of a document

Displays a table of recent versions with version number, author,
date, and commit message. Newest versions are shown first. Defaults
to the last 10 versions; use -n to change the limit or --all to show
every version.`, Usage: "history [-n limit | --all] <path>", MCP: true, Flags: []sdk.Flag{
		{Name: "n", Type: "int", Desc: "Maximum versions to show (default 10)"},
		{Name: "all", Type: "bool", Desc: "Show every version, no limit"},
	},
}

func historyCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(historySpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	// Resolve the effective limit. --all beats -n beats the default.
	limit := flags.Int("n")
	if flags.Bool("all") {
		limit = 0
	} else if limit == 0 {
		limit = defaultHistoryLimit
	}

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
