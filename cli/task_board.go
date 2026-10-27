// task_board.go renders the board view and task tables.

package cli

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/jpl-au/llmd/sdk"
)

// Styles for the board view. Generic table styles (tblHeader, tblCell,
// tblDim, tblBorder) live in styles.go.
var (
	colHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")). // bright blue
			PaddingLeft(1)

	colCount = lipgloss.NewStyle().
			Faint(true)

	emptyCol = lipgloss.NewStyle().
			Faint(true).
			Italic(true).
			PaddingLeft(3)

	flagBlocked = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")). // red
			Padding(0, 1)

	flagHold = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // yellow
			Padding(0, 1)

	flagHuman = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")). // magenta
			Padding(0, 1)
)

// formatBoard renders the board grouped by column. At a terminal each
// column is a styled table; piped, each is a heading with a count and
// one plain task line per task (see plainTaskLine). hiddenDone is how
// many done tasks were left out of tasks; when positive, the done
// column ends with a line saying so.
func formatBoard(ctx sdk.Context, cols []string, tasks []*sdk.Task, hiddenDone int) string {
	specs, err := ctx.Tasks.CheckSpecs(tasks)
	if err != nil {
		slog.Debug("checking task specs for board", "error", err)
	}
	tty := isTTY()

	byStatus := make(map[string][]*sdk.Task)
	for _, t := range tasks {
		byStatus[t.Status] = append(byStatus[t.Status], t)
	}

	var b strings.Builder
	for i, col := range cols {
		if i > 0 && tty {
			b.WriteByte('\n')
		}

		tt := byStatus[col]
		if tty {
			b.WriteString(colHeader.Render(strings.ToUpper(col)) + " " + colCount.Render(fmt.Sprintf("(%d)", len(tt))))
		} else {
			fmt.Fprintf(&b, "%s (%d)", strings.ToUpper(col), len(tt))
		}
		b.WriteByte('\n')

		switch {
		case len(tt) == 0 && tty:
			b.WriteString(emptyCol.Render("no tasks"))
			b.WriteByte('\n')
		case len(tt) > 0 && tty:
			b.WriteString(taskTable(tt, specs))
			b.WriteByte('\n')
		case len(tt) > 0:
			b.WriteString(plainTaskLines(tt, specs))
		}
		if col == "done" && hiddenDone > 0 {
			b.WriteString(emptyCol.Render(fmt.Sprintf("%d more done; use --all or --column done", hiddenDone)))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatTaskTable renders a flat list of tasks: a table at a terminal,
// plain task lines piped.
func formatTaskTable(ctx sdk.Context, tasks []*sdk.Task) string {
	if len(tasks) == 0 {
		return emptyCol.Render("no tasks")
	}
	specs, err := ctx.Tasks.CheckSpecs(tasks)
	if err != nil {
		slog.Debug("checking task specs for table", "error", err)
	}
	if !isTTY() {
		return plainTaskLines(tasks, specs)
	}
	return taskTable(tasks, specs)
}

// plainTaskLine is the one-line form of a task for piped output,
// shared by task list and review so an agent learns it once. Seven
// tab-separated fields, always present, empty when unset:
//
//	key  title  column  P<priority>  assignee  flags  spec
//
// spec is the document path when the spec document exists, else empty.
// A task with no priority shows "-" in the priority field.
func plainTaskLine(t *sdk.Task, hasSpec bool) string {
	spec := ""
	if hasSpec {
		spec = t.Path
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", t.Key, t.Title, t.Status, priorityLabel(t.Priority), t.AssignedTo, t.Flags, spec)
}

// priorityLabel is "P1", "P2" and so on, or "-" for no priority.
func priorityLabel(p int) string {
	if p <= 0 {
		return "-"
	}
	return fmt.Sprintf("P%d", p)
}

// plainTaskLines renders one plainTaskLine per task, each ending in a
// newline.
func plainTaskLines(tasks []*sdk.Task, specs map[string]bool) string {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(plainTaskLine(t, specs[t.Key]))
		b.WriteByte('\n')
	}
	return b.String()
}

// taskTable renders a terminal table using lipgloss.
func taskTable(tasks []*sdk.Task, specs map[string]bool) string {
	t := table.New().
		Headers("ID", "PRI", "TITLE", "ASSIGNED TO", "SPEC", "FLAGS").
		Border(lipgloss.RoundedBorder()).
		BorderRow(false).
		BorderStyle(tblBorder).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeader
			}
			// Colour flags by value
			if col == 5 && row >= 0 && row < len(tasks) {
				switch tk := tasks[row]; {
				case tk.HasFlag("blocked"):
					return flagBlocked
				case tk.HasFlag("hold"):
					return flagHold
				case tk.HasFlag("human"):
					return flagHuman
				}
			}
			// Dim empty cells (assigned to, spec)
			if col == 3 || col == 4 {
				return tblDim
			}
			return tblCell
		})

	for _, tk := range tasks {
		spec := ""
		if specs[tk.Key] {
			spec = tk.Path
		}
		pri := ""
		if tk.Priority > 0 {
			pri = strconv.Itoa(tk.Priority)
		}
		t.Row(
			tk.Key,
			pri,
			tk.Title,
			tk.AssignedTo,
			spec,
			tk.Flags,
		)
	}

	return t.String()
}
