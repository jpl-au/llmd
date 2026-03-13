// task_log.go shows task audit history.

package cli

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jpl-au/llmd/sdk"
)

// taskLog displays the audit history for a task (or all tasks if no key
// is given). Renders a table of events showing timestamp, actor, action,
// and old/new values. Supports -n to limit the number of events.
var taskLogFlags = []sdk.Flag{
	{Name: "n", Type: "int"},
}

func taskLog(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskLogFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task log: %w", err)
	}
	limit := flags.Int("n")
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}

	events, err := ctx.Tasks.Log(key, limit)
	if err != nil {
		return nil, fmt.Errorf("task log: %w", err)
	}

	if len(events) == 0 {
		return sdk.Result{Text: emptyCol.Render("no history"), Data: events}, nil
	}

	// Show TASK column when listing all history
	showSubject := key == ""

	headers := []string{"TIME", "ACTOR", "ACTION", "OLD", "NEW"}
	if showSubject {
		headers = []string{"TIME", "TASK", "ACTOR", "ACTION", "OLD", "NEW"}
	}

	t := table.New().
		Headers(headers...).
		Border(lipgloss.RoundedBorder()).
		BorderRow(false).
		BorderStyle(tblBorder).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeader
			}
			if col == 0 {
				return tblDim
			}
			return tblCell
		})

	for _, e := range events {
		ts := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04")
		if showSubject {
			t.Row(ts, e.Subject, e.Actor, e.Action, e.OldValue, e.NewValue)
		} else {
			t.Row(ts, e.Actor, e.Action, e.OldValue, e.NewValue)
		}
	}

	return sdk.Result{Text: t.String(), Data: events}, nil
}
