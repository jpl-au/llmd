package host

import (
	"github.com/jpl-au/llmd/internal/llmd/rules"
	"github.com/jpl-au/llmd/sdk"
)

// ruleAPI implements [sdk.RuleStore]. Rules are pure filesystem
// operations - no database dependency.
type ruleAPI struct {
	dir string // .llmd/ directory
}

func newRuleAPI(dir string) *ruleAPI {
	return &ruleAPI{dir: dir}
}

func (r *ruleAPI) Show() (map[string]sdk.ColumnRule, error) {
	rs, err := rules.Load(r.dir, "default")
	if err != nil {
		return nil, err
	}
	out := make(map[string]sdk.ColumnRule, len(rs))
	for col, cr := range rs {
		out[col] = sdk.ColumnRule{
			Agent:   cr.Agent,
			Role:    cr.Role,
			Success: cr.Success,
			Failure: cr.Failure,
		}
	}
	return out, nil
}

func (r *ruleAPI) Set(column string, rule sdk.ColumnRule) error {
	rs, err := rules.Load(r.dir, "default")
	if err != nil {
		return err
	}
	rs[column] = rules.ColumnRule{
		Agent:   rule.Agent,
		Role:    rule.Role,
		Success: rule.Success,
		Failure: rule.Failure,
	}
	return rules.Save(r.dir, "default", rs)
}

func (r *ruleAPI) Unset(column string) error {
	rs, err := rules.Load(r.dir, "default")
	if err != nil {
		return err
	}
	cr, ok := rs[column]
	if !ok {
		return nil
	}
	cr.Agent = ""
	cr.Role = ""
	rs[column] = cr
	return rules.Save(r.dir, "default", rs)
}
