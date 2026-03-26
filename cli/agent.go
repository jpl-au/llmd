// agent.go dispatches agent subcommands.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/sdk"
)

var agentSpec = sdk.Command{
	Name: "agent", Desc: `Spawn AI agents to work on tasks in isolated git worktrees.

Quick start:
  llmd agent add claude-code
  llmd task start <task-id> --assign claude-code

Subcommands:
  add <name>                       register an agent
  rm <name>                        remove an agent
  ls                               list registered agents
  config <name>                    show agent configuration
  prompt <name> <role>             show prompt template
  spawn <task-key> <agent>         spawn agent for a task
  runs [--status S] [--task K]     list agent runs
  stop <task-key>                  stop a running agent

Built-in agents: claude-code, gemini, aider
See "llmd guide agent" for full documentation.`, Usage: "agent <subcommand> [options]", MCP: true, MCPName: "agent", Flags: []sdk.Flag{
		{Name: "command", Type: "string", Desc: "Command to run (for custom agents)"},
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

	// stop requires an author (it's an operational action on a running
	// task). add/rm are configuration - use "llmd" as default author
	// so users don't need --author just to set up their tools.
	switch sub {
	case "stop", "spawn", "complete":
		if ctx.Author == "" {
			return nil, fmt.Errorf("agent %s: author required", sub)
		}
	case "add", "rm":
		if ctx.Author == "" {
			ctx.Author = "llmd"
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
	case "spawn":
		// agent spawn <task> <agent> → task start <task> --assign <agent>
		if len(args) < 2 {
			return nil, fmt.Errorf("agent spawn: %w: <task-key> <agent>", sdk.ErrMissingArg)
		}
		return taskStart(ctx, []string{args[0], "--assign", args[1]})
	case "run":
		return agentRun(ctx, args)
	case "complete":
		return agentComplete(ctx, args)
	case "stop":
		return agentStop(ctx, args)
	default:
		return nil, fmt.Errorf("agent: unknown subcommand: %s", sub)
	}
}

var agentAddFlags = []sdk.Flag{
	{Name: "command", Type: "string", Desc: "Command to run (for custom agents)"},
}

func agentAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(agentAddFlags, args)
	if err != nil {
		return nil, fmt.Errorf("agent add: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("agent add: %w: name required\n\nBuilt-in agents: %s\nCustom: llmd agent add <name> --command <binary>",
			sdk.ErrMissingArg, strings.Join(sortedProfileNames(), ", "))
	}

	name := positional[0]
	command := flags.String("command")

	// Look up built-in profile first.
	profile := assets.Agent.Profile(name)

	if profile != nil && command == "" {
		// Known agent - use the built-in profile.
		if err := ctx.Agents.Register(*profile, ctx.Author); err != nil {
			return nil, fmt.Errorf("agent add: %w", err)
		}
		return sdk.Text(fmt.Sprintf(
			"Registered %s\n  Command:  %s\n  Config:   %s\n  Prompts:  %s\n            %s\n\nAssign to a task with: llmd task start <task-id> --assign %s",
			name, profile.Command,
			agents.ConfigPath(name),
			agents.PromptPath(name, "developer"),
			agents.PromptPath(name, "auditor"),
			name,
		)), nil
	}

	if command == "" {
		return nil, fmt.Errorf("agent add: %q is not a built-in agent - use --command to specify the binary\n\nBuilt-in agents: %s",
			name, strings.Join(sortedProfileNames(), ", "))
	}

	// Custom agent.
	cfg := sdk.AgentConfig{
		Name:    name,
		Command: command,
		Args:    []string{"-p", "{{.Prompt}}"},
	}
	if err := ctx.Agents.Register(cfg, ctx.Author); err != nil {
		return nil, fmt.Errorf("agent add: %w", err)
	}
	return sdk.Text(fmt.Sprintf(
		"Registered %s\n  Command:  %s\n  Config:   %s\n  Prompts:  %s\n            %s\n\nAssign to a task with: llmd task start <task-id> --assign %s",
		name, command,
		agents.ConfigPath(name),
		agents.PromptPath(name, "developer"),
		agents.PromptPath(name, "auditor"),
		name,
	)), nil
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
		return sdk.Text("No agents registered\n\nAdd one with: llmd agent add claude-code"), nil
	}

	t := newTable("NAME", "COMMAND")
	for _, c := range cfgs {
		t.Row(c.Name, c.Command)
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

var agentCompleteFlags = []sdk.Flag{
	{Name: "exit-code", Type: "int", Desc: "Process exit code"},
}

func agentComplete(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(agentCompleteFlags, args)
	if err != nil {
		return nil, fmt.Errorf("agent complete: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("agent complete: %w: task key", sdk.ErrMissingArg)
	}
	key := positional[0]
	exitCode := flags.Int("exit-code")

	if err := ctx.Agents.Complete(key, exitCode); err != nil {
		return nil, fmt.Errorf("agent complete: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Completed agent run for task %s (exit %d)", key, exitCode)), nil
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

func sortedProfileNames() []string {
	names := assets.Agent.ProfileNames()
	sort.Strings(names)
	return names
}
