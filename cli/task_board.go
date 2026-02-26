// task_board.go renders the board view and task tables.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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
)

// formatBoard renders the board view grouped by column.
func formatBoard(cols []string, tasks []*sdk.Task) string {
	specs := specExists(tasks)

	byStatus := make(map[string][]*sdk.Task)
	for _, t := range tasks {
		byStatus[t.Status] = append(byStatus[t.Status], t)
	}

	var b strings.Builder
	for i, col := range cols {
		if i > 0 {
			b.WriteByte('\n')
		}

		tt := byStatus[col]
		heading := colHeader.Render(strings.ToUpper(col)) +
			" " + colCount.Render(fmt.Sprintf("(%d)", len(tt)))
		b.WriteString(heading)
		b.WriteByte('\n')

		if len(tt) == 0 {
			b.WriteString(emptyCol.Render("no tasks"))
			b.WriteByte('\n')
		} else {
			b.WriteString(taskTable(tt, specs))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatTaskTable renders a flat table of tasks.
func formatTaskTable(tasks []*sdk.Task) string {
	if len(tasks) == 0 {
		return emptyCol.Render("no tasks")
	}
	return taskTable(tasks, specExists(tasks))
}

// specExists checks which tasks have a document at their path.
// Deduplicates paths so each unique path is queried at most once.
func specExists(tasks []*sdk.Task) map[string]bool {
	// Check each unique path once.
	paths := make(map[string]bool)
	for _, t := range tasks {
		if t.Path == "" {
			continue
		}
		if _, checked := paths[t.Path]; !checked {
			ok, err := sdk.Documents.Exists(t.Path)
			paths[t.Path] = err == nil && ok
		}
	}

	m := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if paths[t.Path] {
			m[t.Key] = true
		}
	}
	return m
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
				f := tasks[row].Flags
				if strings.Contains(f, "blocked") {
					return flagBlocked
				}
				if strings.Contains(f, "hold") {
					return flagHold
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
		t.Row(
			tk.Key,
			strconv.Itoa(tk.Priority),
			tk.Title,
			tk.AssignedTo,
			spec,
			tk.Flags,
		)
	}

	return t.String()
}
