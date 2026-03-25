// agent.go dispatches agent subcommands.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var agentSpec = sdk.Command{
	Name: "agent", Desc: `Manage AI agent configurations and runs.

Subcommands:
  add <name> <command> [args...]   register an agent
  rm <name>                        remove an agent
  ls                               list registered agents
  config <name>                    show agent configuration
  prompt <name> <role>             show prompt template
  runs [--status S] [--task K]     list agent runs
  stop <task-key>                  stop a running agent`, Usage: "agent <subcommand> [options]", MCP: true, MCPName: "agent", Flags: []sdk.Flag{
		{Name: "role", Type: "string", Desc: "Agent role (developer, auditor)"},
		{Name: "budget", Type: "string", Desc: "Max budget per spawn in USD"},
		{Name: "status", Type: "string", Desc: "Filter runs by status"},
		{Name: "task", Type: "string", Desc: "Filter runs by task key"},
		{Name: "agent", Type: "string", Desc: "Filter runs by agent name"},
	},
}

func agentCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("agent: %w", sdk.ErrMissingArg)
	}

	sub := args[0]
	args = args[1:]

	switch sub {
	case "add", "rm", "stop":
		if ctx.Author == "" {
			return nil, fmt.Errorf("agent %s: author required for mutations", sub)
		}
	}

	switch sub {
	case "add":
		return agentAdd(ctx, args)
	case "rm", "remove":
		return agentRm(ctx, args)
	case "ls", "list":
		return agentList(ctx, args)
	case "config":
		return agentConfig(ctx, args)
	case "prompt":
		return agentPrompt(ctx, args)
	case "runs":
		return agentRuns(ctx, args)
	case "stop":
		return agentStop(ctx, args)
	default:
		return nil, fmt.Errorf("agent: unknown subcommand: %s", sub)
	}
}

var agentAddFlags = []sdk.Flag{
	{Name: "role", Type: "string"},
	{Name: "budget", Type: "string"},
}

func agentAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(agentAddFlags, args)
	if err != nil {
		return nil, fmt.Errorf("agent add: %w", err)
	}
	if len(positional) < 2 {
		return nil, fmt.Errorf("agent add: %w: name and command required", sdk.ErrMissingArg)
	}

	name := positional[0]
	command := positional[1]
	var cmdArgs []string
	if len(positional) > 2 {
		cmdArgs = positional[2:]
	}

	var budget float64
	if b := flags.String("budget"); b != "" {
		if _, err := fmt.Sscanf(b, "%f", &budget); err != nil {
			return nil, fmt.Errorf("agent add: invalid budget %q: %w", b, err)
		}
	}

	cfg := sdk.AgentConfig{
		Name:      name,
		Command:   command,
		Args:      cmdArgs,
		Role:      flags.String("role"),
		MaxBudget: budget,
	}

	if err := ctx.Agents.Register(cfg, ctx.Author); err != nil {
		return nil, fmt.Errorf("agent add: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Registered agent %q (%s)", name, command)), nil
}

func agentRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("agent rm: %w: name", sdk.ErrMissingArg)
	}
	if err := ctx.Agents.Remove(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("agent rm: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Removed agent %q", args[0])), nil
}

func agentList(ctx sdk.Context, _ []string) (sdk.Response, error) {
	cfgs, err := ctx.Agents.Agents()
	if err != nil {
		return nil, fmt.Errorf("agent ls: %w", err)
	}
	if len(cfgs) == 0 {
		return sdk.Text("No agents registered"), nil
	}

	t := newTable("NAME", "COMMAND", "ROLE", "BUDGET")
	for _, c := range cfgs {
		cmd := c.Command
		if len(c.Args) > 0 {
			cmd += " " + strings.Join(c.Args, " ")
		}
		role := c.Role
		if role == "" {
			role = "-"
		}
		budget := "-"
		if c.MaxBudget > 0 {
			budget = fmt.Sprintf("$%.2f", c.MaxBudget)
		}
		t.Row(c.Name, cmd, role, budget)
	}

	return sdk.Result{Text: t.String(), Data: cfgs}, nil
}

func agentConfig(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("agent config: %w: name", sdk.ErrMissingArg)
	}
	cfg, err := ctx.Agents.Agent(args[0])
	if err != nil {
		return nil, fmt.Errorf("agent config: %w", err)
	}

	t := newTable("FIELD", "VALUE")
	t.Row("name", cfg.Name)
	t.Row("command", cfg.Command)
	if len(cfg.Args) > 0 {
		t.Row("args", strings.Join(cfg.Args, " "))
	}
	if cfg.Role != "" {
		t.Row("role", cfg.Role)
	}
	if cfg.MaxBudget > 0 {
		t.Row("budget", fmt.Sprintf("$%.2f", cfg.MaxBudget))
	}

	return sdk.Result{Text: t.String(), Data: cfg}, nil
}

func agentPrompt(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("agent prompt: %w: name and role required", sdk.ErrMissingArg)
	}
	name := args[0]
	role := args[1]

	content, path, err := ctx.Agents.Prompt(name, role)
	if err != nil {
		return nil, fmt.Errorf("agent prompt: %w", err)
	}

	header := fmt.Sprintf("# %s/%s (from %s)\n\n", name, role, path)
	return sdk.Result{Text: header + content, Data: map[string]string{
		"name": name, "role": role, "path": path, "content": content,
	}}, nil
}

var agentRunsFlags = []sdk.Flag{
	{Name: "status", Type: "string"},
	{Name: "task", Type: "string"},
	{Name: "agent", Type: "string"},
}

func agentRuns(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(agentRunsFlags, args)
	if err != nil {
		return nil, fmt.Errorf("agent runs: %w", err)
	}

	runs, err := ctx.Agents.Runs(sdk.RunListOpts{
		Status:  flags.String("status"),
		TaskKey: flags.String("task"),
		Agent:   flags.String("agent"),
	})
	if err != nil {
		return nil, fmt.Errorf("agent runs: %w", err)
	}

	if len(runs) == 0 {
		return sdk.Text("No agent runs"), nil
	}

	t := newTable("RUN", "AGENT", "TASK", "STATUS", "BRANCH")
	for _, r := range runs {
		status := r.Status
		if r.Status == sdk.AgentRunning {
			status = fmt.Sprintf("running (pid %d)", r.PID)
		}
		t.Row(r.Key, r.Agent, r.TaskKey, status, r.Branch)
	}

	return sdk.Result{Text: t.String(), Data: runs}, nil
}

func agentStop(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("agent stop: %w: task key", sdk.ErrMissingArg)
	}
	if err := ctx.Agents.Stop(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("agent stop: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Stopped agent for task %s", args[0])), nil
}
