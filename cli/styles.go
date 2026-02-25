// styles.go defines shared lipgloss styles and terminal helpers for CLI output.

package cli

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	tblHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

	tblCell = lipgloss.NewStyle().
		Padding(0, 1)

	tblDim = lipgloss.NewStyle().
		Faint(true).
		Padding(0, 1)

	tblBorder = lipgloss.NewStyle().
			Faint(true)

	// Diff colouring styles, applied line-by-line to unified diff output.
	diffAdded   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	diffRemoved = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	diffHunk    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true)
	diffHeader  = lipgloss.NewStyle().Bold(true)
)

// isTTY reports whether stdout is a terminal.
func isTTY() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}

// newTable creates a styled lipgloss table with standard formatting.
func newTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(tblBorder).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeader
			}
			return tblCell
		})
}
