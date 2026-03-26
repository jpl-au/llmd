// rule.go provides the llmd rule command for managing column rules.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var ruleSpec = sdk.Command{
	Name: "rule", Desc: `Manage column transition and automation rules.

Rules define what happens when a task enters a column: where it goes
on success or failure, and optionally which agent handles the work.
Columns without an agent are manual.

Subcommands:
  show                        display all column rules
  set <column> [flags]        configure a column rule
  unset <column>              remove agent (keep transitions)

Rules are stored in .llmd/rules/default.yaml.
See "llmd guide rule" for full documentation.`, Usage: "rule <subcommand> [options]", MCP: true, Flags: []sdk.Flag{
		{Name: "agent", Type: "string", Desc: "Agent to auto-spawn"},
		{Name: "role", Type: "string", Desc: "Agent role (developer, tester, auditor)"},
		{Name: "success", Type: "string", Desc: "Column on success"},
		{Name: "failure", Type: "string", Desc: "Column on failure"},
	},
}

func ruleCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return ruleShow(ctx, nil)
	}

	sub := args[0]
	args = args[1:]

	switch sub {
	case "show", "ls":
		return ruleShow(ctx, args)
	case "set":
		return ruleSet(ctx, args)
	case "unset":
		return ruleUnset(ctx, args)
	default:
		return nil, fmt.Errorf("rule: unknown subcommand: %s", sub)
	}
}

func ruleShow(ctx sdk.Context, _ []string) (sdk.Response, error) {
	rs, err := ctx.Rules.Show()
	if err != nil {
		return nil, fmt.Errorf("rule show: %w", err)
	}
	if len(rs) == 0 {
		return sdk.Text("No rules configured\n\nSet one with: llmd rule set code --agent claude-code --role developer --success test --failure blocked"), nil
	}

	// Sort columns for stable output.
	var cols []string
	for col := range rs {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	t := newTable("COLUMN", "AGENT", "ROLE", "SUCCESS", "FAILURE")
	for _, col := range cols {
		r := rs[col]
		agent := r.Agent
		if agent == "" {
			agent = "-"
		}
		role := r.Role
		if role == "" {
			role = "-"
		}
		t.Row(col, agent, role, r.Success, r.Failure)
	}

	return sdk.Result{Text: t.String(), Data: rs}, nil
}

var ruleSetFlags = []sdk.Flag{
	{Name: "agent", Type: "string"},
	{Name: "role", Type: "string"},
	{Name: "success", Type: "string"},
	{Name: "failure", Type: "string"},
}

func ruleSet(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(ruleSetFlags, args)
	if err != nil {
		return nil, fmt.Errorf("rule set: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("rule set: %w: column name", sdk.ErrMissingArg)
	}

	column := positional[0]

	// Load existing rule to preserve values not being set.
	existing, _ := ctx.Rules.Show()
	rule := sdk.ColumnRule{}
	if existing != nil {
		rule = existing[column]
	}

	if v := flags.String("agent"); v != "" {
		rule.Agent = v
	}
	if v := flags.String("role"); v != "" {
		rule.Role = v
	}
	if v := flags.String("success"); v != "" {
		rule.Success = v
	}
	if v := flags.String("failure"); v != "" {
		rule.Failure = v
	}

	if err := ctx.Rules.Set(column, rule); err != nil {
		return nil, fmt.Errorf("rule set: %w", err)
	}

	var parts []string
	if rule.Agent != "" {
		parts = append(parts, fmt.Sprintf("agent=%s", rule.Agent))
	}
	if rule.Role != "" {
		parts = append(parts, fmt.Sprintf("role=%s", rule.Role))
	}
	if rule.Success != "" {
		parts = append(parts, fmt.Sprintf("success=%s", rule.Success))
	}
	if rule.Failure != "" {
		parts = append(parts, fmt.Sprintf("failure=%s", rule.Failure))
	}

	return sdk.Text(fmt.Sprintf("Set rule for %s: %s", column, strings.Join(parts, " "))), nil
}

func ruleUnset(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rule unset: %w: column name", sdk.ErrMissingArg)
	}

	if err := ctx.Rules.Unset(args[0]); err != nil {
		return nil, fmt.Errorf("rule unset: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Removed agent from %s (transitions preserved)", args[0])), nil
}
