// Package ui provides view models for llmd's user interface layers.
// Each domain has its own file with view model types and constructors.
// Rendering is handled by sub-packages (terminal/, http/) that
// consume these models.
package ui

import "github.com/jpl-au/llmd/sdk"

// RuleView is the view model for a column rule.
type RuleView struct {
	Column  string
	Agent   string
	Role    string
	Success string
	Failure string
	Manual  bool
}

// NewRuleView builds a RuleView from an SDK ColumnRule.
func NewRuleView(column string, r sdk.ColumnRule) RuleView {
	return RuleView{
		Column:  column,
		Agent:   r.Agent,
		Role:    r.Role,
		Success: r.Success,
		Failure: r.Failure,
		Manual:  r.Agent == "",
	}
}

// NewRuleViews builds a sorted slice of RuleViews from a rule map.
func NewRuleViews(rs map[string]sdk.ColumnRule) []RuleView {
	// Collect and sort keys for stable output.
	keys := make([]string, 0, len(rs))
	for col := range rs {
		keys = append(keys, col)
	}
	sortStrings(keys)

	views := make([]RuleView, len(keys))
	for i, col := range keys {
		views[i] = NewRuleView(col, rs[col])
	}
	return views
}

// sortStrings sorts a string slice in place (avoids importing sort
// in the ui package for a single use).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
