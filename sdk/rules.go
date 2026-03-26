package sdk

// RuleStore manages column transition and automation rules.
//
// Rules live on disk as YAML files in .llmd/rules/. Each column can
// define success/failure transitions and an optional agent for
// automation. Columns without an agent entry are manual - a human
// triggers the work, but the transitions still follow the rule.
type RuleStore interface {
	// Show returns the current rule set.
	Show() (map[string]ColumnRule, error)

	// Set updates or creates a rule for a column.
	Set(column string, rule ColumnRule) error

	// Unset removes the agent from a column's rule, keeping
	// the transitions intact.
	Unset(column string) error
}

// ColumnRule defines the behaviour for a single board column.
type ColumnRule struct {
	Agent   string `json:"agent,omitempty" yaml:"agent,omitempty"`
	Role    string `json:"role,omitempty" yaml:"role,omitempty"`
	Success string `json:"success" yaml:"success"`
	Failure string `json:"failure" yaml:"failure"`
}
