// status.go renders a single-screen dashboard of the store.

package cli

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jpl-au/llmd/sdk"
)

var statusSpec = sdk.Command{
	Name: "status", Desc: `Overview dashboard showing recent documents, tasks, and activity

Shows recent documents, task board summary, and latest task events
in a single view. Use -n to control how many items per section.`, Usage: "status [-n limit]", Flags: []sdk.Flag{
		{Name: "n", Type: "int", Desc: "Items per section (default 5)"},
	},
}

var (
	sectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginTop(1)

	countBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true)

	countLabel = lipgloss.NewStyle().
			Faint(true)
)

// status renders a single-screen dashboard showing recent documents,
// task board summary, and activity feed. Falls back to plain text when
// output is not a terminal.
func status(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(statusSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}
	limit := flags.Int("n")
	if limit == 0 {
		limit = 5
	}

	docs, err := ctx.Documents.List("", sdk.ListOpts{Sort: "time"})
	if err != nil {
		slog.Warn("listing documents", "error", err)
	}
	if len(docs) > limit {
		docs = docs[:limit]
	}

	cols, err := ctx.Tasks.Columns()
	if err != nil {
		slog.Warn("listing columns", "error", err)
	}
	tasks, err := ctx.Tasks.List(sdk.TaskListOpts{})
	if err != nil {
		slog.Warn("listing tasks", "error", err)
	}

	byStatus := make(map[string]int)
	for _, t := range tasks {
		byStatus[t.Status]++
	}

	var activity []sdk.Activity
	if ctx.Activities != nil {
		activity, err = ctx.Activities.Recent(limit)
		if err != nil {
			slog.Warn("listing activity", "error", err)
		}
	}

	data := map[string]any{
		"recent_documents": docs,
		"task_summary":     byStatus,
		"recent_activity":  activity,
	}

	if !isTTY() {
		return sdk.Result{Text: plainStatus(docs, cols, byStatus, activity), Data: data}, nil
	}

	var b strings.Builder

	// Recent documents
	b.WriteString(sectionTitle.Render("RECENT DOCUMENTS"))
	b.WriteByte('\n')
	if len(docs) == 0 {
		b.WriteString(emptyCol.Render("no documents"))
		b.WriteByte('\n')
	} else {
		t := newTable("PATH", "VER", "AUTHOR", "DATE")
		for _, d := range docs {
			date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
			t.Row(d.Path, strconv.Itoa(d.Version), d.Author, date)
		}
		b.WriteString(t.String())
		b.WriteByte('\n')
	}

	// Task board summary
	b.WriteString(sectionTitle.Render("TASK BOARD"))
	b.WriteByte('\n')
	if len(cols) == 0 {
		b.WriteString(emptyCol.Render("no columns"))
		b.WriteByte('\n')
	} else {
		b.WriteString("  ")
		for i, col := range cols {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(countBadge.Render(strconv.Itoa(byStatus[col])))
			b.WriteByte(' ')
			b.WriteString(countLabel.Render(col))
		}
		b.WriteByte('\n')
	}

	// Recent activity
	b.WriteString(sectionTitle.Render("RECENT ACTIVITY"))
	b.WriteByte('\n')
	if len(activity) == 0 {
		b.WriteString(emptyCol.Render("no activity"))
		b.WriteByte('\n')
	} else {
		t := table.New().
			Headers("TIME", "TYPE", "SUBJECT", "ACTION", "DETAIL").
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
		for _, e := range activity {
			ts := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04")
			t.Row(ts, e.Type, e.Subject, actionLabel(e.Action), e.Detail)
		}
		b.WriteString(t.String())
		b.WriteByte('\n')
	}

	return sdk.Result{Text: b.String(), Data: data}, nil
}

// actionLabel prefixes an action with an emoji.
func actionLabel(action string) string {
	switch {
	case action == "written", action == "created":
		return "✨ " + action
	case action == "deleted":
		return "🗑️ " + action
	case action == "restored":
		return "♻️ " + action
	case action == "moved":
		return "📦 " + action
	case action == "tagged":
		return "🏷️ " + action
	case action == "untagged":
		return "🏷️ " + action
	case action == "linked":
		return "🔗 " + action
	case action == "unlinked":
		return "🔗 " + action
	case action == "flagged", action == "unflagged":
		return "🚩 " + action
	case strings.HasPrefix(action, "edited:"):
		return "✏️ " + action
	default:
		return action
	}
}

// plainStatus renders a text-only dashboard for piped output.
func plainStatus(docs []sdk.Doc, cols []string, counts map[string]int, activity []sdk.Activity) string {
	var b strings.Builder
	b.WriteString("Recent documents:\n")
	for _, d := range docs {
		fmt.Fprintf(&b, "  %s v%d %s\n", d.Path, d.Version, d.Author)
	}
	b.WriteString("\nTask board:\n")
	for _, col := range cols {
		fmt.Fprintf(&b, "  %s: %d\n", col, counts[col])
	}
	b.WriteString("\nRecent activity:\n")
	for _, e := range activity {
		ts := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "  %s %s %s %s\n", ts, e.Type, e.Subject, e.Action)
	}
	return b.String()
}
