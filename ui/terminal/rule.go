// Package terminal provides lipgloss-based rendering for the CLI.
package terminal

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jpl-au/llmd/ui"
)

var (
	ruleColumn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	ruleAgent   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	ruleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ruleFailure = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	ruleDim     = lipgloss.NewStyle().Faint(true)
	ruleLabel   = lipgloss.NewStyle().Faint(true).Width(10)
)

// RenderRule renders a single rule as a coloured tree.
func RenderRule(v ui.RuleView) string {
	header := ruleColumn.Render(v.Column)
	if !v.Manual {
		header += " " + ruleAgent.Render(fmt.Sprintf("[%s, %s]", v.Agent, v.Role))
	} else {
		header += " " + ruleDim.Render("[manual]")
	}

	success := ruleSuccess.Render("success:") + " " + ruleColumn.Render(v.Success) + " " + ruleSuccess.Render("\u2192")
	failure := ruleFailure.Render("failure:") + " " + ruleColumn.Render(v.Failure) + " " + ruleFailure.Render("\u2190")

	return fmt.Sprintf("%s\n %s %s\n %s %s",
		header,
		ruleDim.Render("\u251c\u2500"), success,
		ruleDim.Render("\u2514\u2500"), failure,
	)
}

// RenderRules renders multiple rules as a coloured tree list.
func RenderRules(views []ui.RuleView) string {
	var parts []string
	for _, v := range views {
		parts = append(parts, RenderRule(v))
	}
	return strings.Join(parts, "\n\n")
}
