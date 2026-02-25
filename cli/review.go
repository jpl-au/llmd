// review.go shows pending tasks with inline spec previews and links.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jpl-au/llmd/sdk"
)

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

func review(_ sdk.Context, args []string) (sdk.Response, error) {
	var column string
	limit := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("review: --column requires a value")
			}
			column = args[i]
		case "-n":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("review: -n requires a value")
			}
			limit, _ = strconv.Atoi(args[i])
		default:
			if column == "" && !strings.HasPrefix(args[i], "-") {
				column = args[i]
			}
		}
	}

	tasks, err := sdk.Tasks.List(sdk.TaskListOpts{Status: column})
	if err != nil {
		return nil, fmt.Errorf("review: %w", err)
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
		data[i].Spec = specPreview(t.Path, 5)
		data[i].Links = linkedDocs(t.Path)
	}

	if !isTTY() {
		return sdk.Result{Text: plainReview(tasks), Data: data}, nil
	}

	if len(tasks) == 0 {
		return sdk.Result{Text: emptyCol.Render("no tasks"), Data: data}, nil
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

	return sdk.Result{Text: b.String(), Data: data}, nil
}

// specPreview returns the first n non-blank lines of a task's spec,
// skipping the title heading.
func specPreview(path string, n int) string {
	if path == "" {
		return ""
	}
	body, err := sdk.Documents.Read(path, 0)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(body), "\n")
	var preview []string
	for _, line := range lines {
		if len(preview) == 0 && strings.HasPrefix(line, "# ") {
			continue
		}
		if strings.TrimSpace(line) == "" && len(preview) == 0 {
			continue
		}
		preview = append(preview, line)
		if len(preview) >= n {
			break
		}
	}
	return strings.Join(preview, "\n")
}

// linkedDocs returns outgoing links for a task's spec path.
func linkedDocs(path string) []sdk.Link {
	if path == "" || sdk.Links == nil {
		return nil
	}
	links, err := sdk.Links.List(path, "out")
	if err != nil {
		return nil
	}
	return links
}

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
