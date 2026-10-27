// styles.go defines shared lipgloss styles and terminal helpers for CLI output.

package cli

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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

// grid is a table that knows its audience. At a terminal it renders
// as a bordered, styled lipgloss table. Piped, as it is for a model
// running llmd in a shell, it renders as tab-separated lines with the
// headers on the first line: nothing to draw, nothing to parse past.
type grid struct {
	headers []string
	rows    [][]string
	style   func(row, col int) lipgloss.Style
}

// newTable creates a grid with the standard header and cell styles.
func newTable(headers ...string) *grid {
	return &grid{headers: headers, style: func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return tblHeader
		}
		return tblCell
	}}
}

// Style replaces the per-cell style used at a terminal.
func (g *grid) Style(fn func(row, col int) lipgloss.Style) *grid {
	g.style = fn
	return g
}

// Row appends a row.
func (g *grid) Row(cells ...string) {
	g.rows = append(g.rows, cells)
}

// String renders the grid for the current stdout.
func (g *grid) String() string {
	if !isTTY() {
		return g.plain()
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderRow(false).
		BorderStyle(tblBorder).
		Headers(g.headers...).
		StyleFunc(g.style)
	for _, r := range g.rows {
		t.Row(r...)
	}
	return t.String()
}

// plain renders the headers and rows as tab-separated lines.
func (g *grid) plain() string {
	var b strings.Builder
	b.WriteString(strings.Join(g.headers, "\t"))
	for _, r := range g.rows {
		b.WriteByte('\n')
		b.WriteString(strings.Join(r, "\t"))
	}
	return b.String()
}
