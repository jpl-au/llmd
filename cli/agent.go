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
  llmd task start <task-id> --agent claude-code

Subcommands:
  add <name>                       register an agent
  rm <name>                        remove an agent
  ls                               list registered agents
  config <name>                    show agent configuration
  prompt <name> <role>             show prompt template
  spawn <task-key> <agent>         spawn agent for a task [--no-worktree] [--budget usd]
  runs [--status S] [--task K]     list agent runs
  complete <task-key> [--exit N]   record an agent's exit
  stop <task-key>                  stop a running agent
  run <task-key> --worktree <dir>  internal: run the agent process

Built-in agents: claude-code, gemini, aider
See "llmd guide agent" for full documentation.`, Usage: "agent <subcommand> [options]", MCP: true, MCPName: "agent", Brief: `Run AI agents on tasks in isolated git worktrees. Built-in: claude-code, gemini, aider.
Subcommands: add <name> [--command bin], rm <name>, ls, config <name>, prompt <name> <role>, spawn <task> <agent> [--no-worktree], runs [--status s] [--task k], complete <task> --exit-code n, stop <task>. Or: task start <id> --agent <name>.`, Flags: []sdk.Flag{
		{Name: "command", Type: "string", Desc: "Command to run (for custom agents)"},
		{Name: "status", Type: "string", Desc: "Filter runs by status"},
		{Name: "task", Type: "string", Desc: "Filter runs by task key"},
		{Name: "agent", Type: "string", Desc: "Filter runs by agent name"},
	},
}

func agentCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent: a subcommand is required. Run llmd agent --help to list them.")
	}

	sub := args[0]
	args = args[1:]

	// spawn, complete and stop act on a task and need an author. add
	// and rm only write configuration files.
	switch sub {
	case "stop", "spawn", "complete":
		if ctx.Author == "" {
			return nil, sdk.AuthorRequired("agent " + sub)
		}
	case "add", "rm":
		// Configuration, like llmd config: nothing is recorded, so no
		// author is needed.
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
		// agent spawn <task> <agent> → task start <task> --agent <agent>
		if len(args) < 2 {
			return nil, sdk.Errorf(sdk.ErrMissingArg, "agent spawn: a task key and an agent are required.")
		}
		return taskStart(ctx, append([]string{args[0], "--agent", args[1]}, args[2:]...))
	case "run":
		return agentRun(ctx, args)
	case "complete":
		return agentComplete(ctx, args)
	case "stop":
		return agentStop(ctx, args)
	default:
		return nil, sdk.Errorf(sdk.ErrInvalidArg, "agent: unknown subcommand %s. Run llmd agent --help to list them.", sub)
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
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent add: an agent name is required. The built-in agents are %s, or add your own with --command <binary>.",
			strings.Join(sortedProfileNames(), ", "))
	}

	name := positional[0]
	command := flags.String("command")

	// Look up built-in profile first.
	profile := assets.Agent.Profile(name)

	if profile != nil && command == "" {
		// Known agent - use the built-in profile.
		if err := ctx.Agents.Register(*profile); err != nil {
			return nil, fmt.Errorf("agent add: %w", err)
		}
		return sdk.Text(fmt.Sprintf(
			"Registered %s\n  Command:  %s\n  Config:   %s\n  Prompts:  %s\n            %s\n\nSpawn it on a task with: llmd task start <task-id> --agent %s",
			name, profile.Command,
			agents.ConfigPath(name),
			agents.PromptPath(name, "developer"),
			agents.PromptPath(name, "auditor"),
			name,
		)), nil
	}

	if command == "" {
		return nil, sdk.Errorf(sdk.ErrInvalidArg, "agent add: %s is not a built-in agent. Pass --command <binary>, or use one of %s.",
			name, strings.Join(sortedProfileNames(), ", "))
	}

	// Custom agent.
	cfg := sdk.AgentConfig{
		Name:    name,
		Command: command,
		Args:    []string{"-p", "{{.Prompt}}"},
	}
	if err := ctx.Agents.Register(cfg); err != nil {
		return nil, fmt.Errorf("agent add: %w", err)
	}
	return sdk.Text(fmt.Sprintf(
		"Registered %s\n  Command:  %s\n  Config:   %s\n  Prompts:  %s\n            %s\n\nSpawn it on a task with: llmd task start <task-id> --agent %s",
		name, command,
		agents.ConfigPath(name),
		agents.PromptPath(name, "developer"),
		agents.PromptPath(name, "auditor"),
		name,
	)), nil
}

func agentRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent rm: an agent name is required.")
	}
	if err := ctx.Agents.Remove(args[0]); err != nil {
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

	t := newTable(ctx.TTY, "NAME", "COMMAND")
	for _, c := range cfgs {
		t.Row(c.Name, c.Command)
	}

	return sdk.Result{Text: t.String(), Data: cfgs}, nil
}

func agentConfig(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent config: an agent name is required.")
	}
	cfg, err := ctx.Agents.Agent(args[0])
	if err != nil {
		return nil, fmt.Errorf("agent config: %w", err)
	}

	t := newTable(ctx.TTY, "FIELD", "VALUE")
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
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent prompt: an agent name and a role are required.")
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

	t := newTable(ctx.TTY, "RUN", "AGENT", "TASK", "STATUS", "BRANCH")
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
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent complete: a task key is required.")
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
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent stop: a task key is required.")
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
