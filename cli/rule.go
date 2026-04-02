// rule.go provides the llmd rule command for managing column rules.

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/ui"
	"github.com/jpl-au/llmd/ui/terminal"
)

var ruleSpec = sdk.Command{
	Name: "rule", Desc: `Manage column transition and automation rules.

Rules define what happens when a task enters a column: where it goes
on success or failure, and optionally which agent handles the work.
Columns without an agent are manual.

Subcommands:
  list                        display all column rules
  set <column> [flags]        configure a column rule
  unset <column>              remove agent (keep transitions)

Rules are stored in .llmd/rules/default.yaml.
See "llmd guide rule" for full documentation.`, Usage: "rule <subcommand> [options]", MCP: true, Flags: []sdk.Flag{
		{Name: "agent", Type: "string", Desc: "Agent to auto-spawn"},
		{Name: "role", Type: "string", Desc: "Agent role (developer, tester, auditor)"},
		{Name: "success", Type: "string", Desc: "Column on success"},
		{Name: "failure", Type: "string", Desc: "Column on failure"},
		{Name: "resume", Type: "bool", Desc: "Resume previous session when auto-spawning"},
	},
}

func ruleCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return ruleList(ctx, nil)
	}

	sub := args[0]
	args = args[1:]

	switch sub {
	case "list", "ls", "show":
		return ruleList(ctx, args)
	case "set":
		return ruleSet(ctx, args)
	case "unset":
		return ruleUnset(ctx, args)
	default:
		return nil, fmt.Errorf("rule: unknown subcommand: %s", sub)
	}
}

func ruleList(ctx sdk.Context, _ []string) (sdk.Response, error) {
	rs, err := ctx.Rules.Show()
	if err != nil {
		return nil, fmt.Errorf("rule list: %w", err)
	}
	if len(rs) == 0 {
		return sdk.Text("No rules configured\n\nSet one with: llmd rule set code --agent claude-code --role developer"), nil
	}

	views := ui.NewRuleViews(rs)
	return sdk.Result{Text: terminal.RenderRules(views), Data: rs}, nil
}

var ruleSetFlags = []sdk.Flag{
	{Name: "agent", Type: "string"},
	{Name: "role", Type: "string"},
	{Name: "success", Type: "string"},
	{Name: "failure", Type: "string"},
	{Name: "resume", Type: "bool", Desc: "Resume previous session when auto-spawning"},
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
	if flags.Bool("resume") {
		rule.Resume = true
	}

	if err := ctx.Rules.Set(column, rule); err != nil {
		return nil, fmt.Errorf("rule set: %w", err)
	}

	view := ui.NewRuleView(column, rule)
	return sdk.Result{Text: terminal.RenderRule(view), Data: rule}, nil
}

func ruleUnset(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("rule unset: %w: column name", sdk.ErrMissingArg)
	}

	column := args[0]
	if err := ctx.Rules.Unset(column); err != nil {
		return nil, fmt.Errorf("rule unset: %w", err)
	}

	rs, _ := ctx.Rules.Show()
	rule := rs[column]

	view := ui.NewRuleView(column, rule)
	return sdk.Result{Text: terminal.RenderRule(view), Data: rule}, nil
}
