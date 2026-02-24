// styles.go defines shared lipgloss styles for CLI table output.

package cli

import (
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
)

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
