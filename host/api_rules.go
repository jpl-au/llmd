package host

import (
	"github.com/jpl-au/llmd/engine/rules"
	"github.com/jpl-au/llmd/sdk"
)

// ruleAPI implements [sdk.RuleStore]. Rules are pure filesystem
// operations - no database dependency.
type ruleAPI struct {
	dir   string                  // .llmd/ directory
	agent func(name string) error // nil when the named agent is registered
}

func newRuleAPI(dir string, agent func(name string) error) *ruleAPI {
	return &ruleAPI{dir: dir, agent: agent}
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
			Resume:  cr.Resume,
		}
	}
	return out, nil
}

func (r *ruleAPI) Set(column string, rule sdk.ColumnRule) error {
	// A rule naming an unknown agent would do nothing until a task
	// entered the column, and then fail with only a log line: refuse it
	// now.
	if rule.Agent != "" && r.agent != nil && r.agent(rule.Agent) != nil {
		return sdk.Errorf(sdk.ErrNotFound, "there is no agent %s. Run llmd agent add %s first.", rule.Agent, rule.Agent)
	}
	rs, err := rules.Load(r.dir, "default")
	if err != nil {
		return err
	}
	rs[column] = rules.ColumnRule{
		Agent:   rule.Agent,
		Role:    rule.Role,
		Success: rule.Success,
		Failure: rule.Failure,
		Resume:  rule.Resume,
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
		return sdk.Errorf(sdk.ErrNotFound, "column %s has no rule.", column)
	}
	cr.Agent = ""
	cr.Role = ""
	rs[column] = cr
	return rules.Save(r.dir, "default", rs)
}
