// review.go shows pending tasks with inline spec previews and links.

package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jpl-au/llmd/sdk"
)

var reviewSpec = sdk.Command{
	Name: "review", Desc: `Review pending tasks with inline context and spec previews

Shows task metadata, spec document previews, and linked documents
for each task. Filter by column to focus on specific workflow stages.
Without a column, done tasks and tasks flagged human are left out;
--all includes them.`, Usage: "review [--column name] [-n limit] [--all]", Flags: []sdk.Flag{
		{Name: "column", Type: "string", Desc: "Filter by column"},
		{Name: "n", Type: "int", Desc: "Maximum tasks to show"},
		{Name: "all", Type: "bool", Desc: "Include done and human tasks"},
	},
}

var (
	reviewTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	reviewKey = lipgloss.NewStyle().
			Faint(true)

	reviewMeta = lipgloss.NewStyle().
			Faint(true).
			PaddingLeft(2)

	reviewSnippet = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("7"))

	reviewSep = lipgloss.NewStyle().
			Faint(true)
)

// review shows pending tasks with inline spec previews and linked
// documents, giving a quick picture of what needs attention.
func review(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(reviewSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}
	column := flags.String("column")
	limit := flags.Int("n")
	if column == "" && len(positional) > 0 {
		column = positional[0]
	}

	tasks, err := ctx.Tasks.List(sdk.TaskListOpts{Status: column})
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
	}

	// The review is what is pending for whoever is asking. Without an
	// explicit column, finished work and work flagged for the user are
	// left out; a count at the end says what was skipped.
	var note string
	if column == "" && !flags.Bool("all") {
		var hidden []string
		tasks, hidden = pendingOnly(tasks)
		if len(hidden) > 0 {
			note = fmt.Sprintf("(%s not shown; use --all)", strings.Join(hidden, ", "))
		}
	}

	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	type entry struct {
		Task  *sdk.Task  `json:"task"`
		Spec  string     `json:"spec,omitempty"`
		Links []sdk.Link `json:"links,omitempty"`
	}
	data := make([]entry, len(tasks))
	for i, t := range tasks {
		data[i] = entry{Task: t}
		if spec, err := ctx.Documents.Preview(t.Path, 5); err != nil {
			slog.Debug("preview failed", "path", t.Path, "error", err)
		} else {
			data[i].Spec = spec
		}
		data[i].Links = linkedDocs(ctx, t.Path)
	}

	if !isTTY() {
		text := plainReview(tasks)
		if note != "" {
			text += note + "\n"
		}
		return sdk.Result{Text: text, Data: data}, nil
	}

	if len(tasks) == 0 {
		text := emptyCol.Render("no tasks")
		if note != "" {
			text += "\n" + reviewSep.Render(note)
		}
		return sdk.Result{Text: text, Data: data}, nil
	}

	var b strings.Builder
	sep := reviewSep.Render(strings.Repeat("─", 40))

	for i, e := range data {
		if i > 0 {
			b.WriteString(sep)
			b.WriteByte('\n')
		}

		// Header: key + title + column
		b.WriteString(reviewKey.Render(e.Task.Key))
		b.WriteString("  ")
		b.WriteString(reviewTitle.Render(e.Task.Title))
		b.WriteString("  ")
		b.WriteString(colHeader.Render(strings.ToUpper(e.Task.Status)))
		b.WriteByte('\n')

		// Metadata
		meta := fmt.Sprintf("Priority: %d", e.Task.Priority)
		if e.Task.AssignedTo != "" {
			meta += "  Assigned: " + e.Task.AssignedTo
		}
		if e.Task.Flags != "" {
			meta += "  Flags: " + e.Task.Flags
		}
		b.WriteString(reviewMeta.Render(meta))
		b.WriteByte('\n')

		// Spec preview
		if e.Spec != "" {
			b.WriteString(reviewMeta.Render("Spec:"))
			b.WriteByte('\n')
			b.WriteString(reviewSnippet.Render(e.Spec))
			b.WriteByte('\n')
		}

		// Linked documents
		if len(e.Links) > 0 {
			b.WriteString(reviewMeta.Render("Links:"))
			b.WriteByte('\n')
			for _, l := range e.Links {
				label := l.To
				if l.Label != "" {
					label += " (" + l.Label + ")"
				}
				b.WriteString(reviewSnippet.Render(label))
				b.WriteByte('\n')
			}
		}
	}

	b.WriteByte('\n')
	b.WriteString(reviewSep.Render(fmt.Sprintf("%d tasks", len(tasks))))
	b.WriteByte('\n')
	if note != "" {
		b.WriteString(reviewSep.Render(note))
		b.WriteByte('\n')
	}

	return sdk.Result{Text: b.String(), Data: data}, nil
}

// pendingOnly drops done tasks and tasks flagged human. It returns the
// rest and a description of what was dropped, such as "3 done, 1
// human", empty when nothing was.
func pendingOnly(tasks []*sdk.Task) ([]*sdk.Task, []string) {
	var done, human int
	kept := make([]*sdk.Task, 0, len(tasks))
	for _, t := range tasks {
		switch {
		case t.Status == "done":
			done++
		case t.HasFlag("human"):
			human++
		default:
			kept = append(kept, t)
		}
	}
	var hidden []string
	if done > 0 {
		hidden = append(hidden, fmt.Sprintf("%d done", done))
	}
	if human > 0 {
		hidden = append(hidden, fmt.Sprintf("%d human", human))
	}
	return kept, hidden
}

// linkedDocs returns outgoing links for a task's spec path.
func linkedDocs(ctx sdk.Context, path string) []sdk.Link {
	if path == "" || ctx.Links == nil {
		return nil
	}
	links, err := ctx.Links.List(path, "out")
	if err != nil {
		return nil
	}
	return links
}

// plainReview renders a tab-separated task listing for piped output.
func plainReview(tasks []*sdk.Task) string {
	var b strings.Builder
	for _, t := range tasks {
		fmt.Fprintf(&b, "%s\t%s\t%s\tP%d\t%s",
			t.Key, t.Title, t.Status, t.Priority, t.AssignedTo)
		if t.Flags != "" {
			fmt.Fprintf(&b, "\t%s", t.Flags)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
