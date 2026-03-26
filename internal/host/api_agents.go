package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/sdk"
)

// agentAPI implements [sdk.AgentStore] by delegating data operations to
// the internal agents package and handling orchestration (worktree
// creation, context assembly, subprocess spawning) in the bridge layer.
// This follows the same pattern as taskAPI handling git operations.
type agentAPI struct {
	ctx   context.Context
	store *llmd.Store
}

func newAgentAPI(store *llmd.Store, ctx context.Context) *agentAPI {
	return &agentAPI{ctx: ctx, store: store}
}

func agentErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, agents.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, agents.ErrRunNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, agents.ErrRunning):
		return fmt.Errorf("%w: %w", sdk.ErrAgentRunning, err)
	default:
		return err
	}
}

func runToSDK(r *agents.Run) *sdk.AgentRun {
	return &sdk.AgentRun{
		Key:          r.Key,
		TaskKey:      r.TaskKey,
		Agent:        r.Agent,
		Branch:       r.Branch,
		Worktree:     r.Worktree,
		Status:       r.Status,
		PID:          r.PID,
		ExitCode:     r.ExitCode,
		MonetaryCost: r.MonetaryCost,
		InputTokens:  r.InputTokens,
		OutputTokens: r.OutputTokens,
		Model:        r.Model,
		Author:       r.Author,
		StartedAt:    r.StartedAt,
		StoppedAt:    r.StoppedAt,
	}
}

// Register adds or updates an agent configuration.
func (a *agentAPI) Register(cfg sdk.AgentConfig, author string) error {
	return a.store.Agents.Register(a.ctx, cfg, author)
}

// Remove deletes an agent configuration by name.
func (a *agentAPI) Remove(name, author string) error {
	return a.store.Agents.Remove(a.ctx, name, author)
}

// Agent returns a single agent configuration by name.
func (a *agentAPI) Agent(name string) (*sdk.AgentConfig, error) {
	cfg, err := a.store.Agents.Agent(a.ctx, name)
	if err != nil {
		return nil, agentErr(err)
	}
	return cfg, nil
}

// Agents returns all registered agent configurations.
func (a *agentAPI) Agents() ([]sdk.AgentConfig, error) {
	return a.store.Agents.Agents(a.ctx)
}

// Spawn creates a worktree, assembles context, and starts the agent
// process for the given task.
func (a *agentAPI) Spawn(taskKey, agent, author string, opts sdk.SpawnOpts) (*sdk.AgentRun, error) {
	// Read the task.
	t, err := a.store.Tasks.Read(a.ctx, taskKey)
	if err != nil {
		return nil, taskErr(err)
	}

	// Check dependencies are satisfied.
	ready, err := a.store.Tasks.Ready(a.ctx, taskKey)
	if err != nil {
		return nil, fmt.Errorf("checking readiness: %w", err)
	}
	if !ready {
		return nil, fmt.Errorf("%w: %s", sdk.ErrNotReady, taskKey)
	}

	// Look up agent configuration.
	cfg, err := a.store.Agents.Agent(a.ctx, agent)
	if err != nil {
		return nil, agentErr(err)
	}

	// Auto-detect role from column. Tasks in review need an auditor,
	// Role resolution: SpawnOpts > AgentConfig > auto-detection.
	if opts.Role != "" {
		cfg.Role = opts.Role
	}
	if cfg.Role == "" {
		switch t.Status {
		case "review":
			cfg.Role = "auditor"
		default:
			cfg.Role = "developer"
		}
	}

	// Resolve the command binary.
	cmdPath, err := exec.LookPath(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("agent command %q not found: %w", cfg.Command, err)
	}

	// Ensure git is available - worktrees are required for agent isolation.
	if err := sdk.Git.Available(); err != nil {
		return nil, fmt.Errorf("git required for agent spawn: %w", err)
	}

	// Determine the branch name.
	branch := t.Branch
	needsBranch := branch == ""
	if needsBranch {
		branch = "task/" + branchSlug(t.Title)
	}

	// Auditors review existing work - they don't need an isolated
	// worktree. They run from the main working directory and read
	// the developer's diff via task commands.
	var absWorktree string
	if cfg.Role == "auditor" {
		absWorktree, err = filepath.Abs(".")
		if err != nil {
			return nil, fmt.Errorf("resolving working directory: %w", err)
		}
	} else {
		// Create worktree (and branch if needed).
		worktreeDir := filepath.Join(".llmd", "worktrees", taskKey)
		absWorktree, err = filepath.Abs(worktreeDir)
		if err != nil {
			return nil, fmt.Errorf("resolving worktree path: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(absWorktree), 0755); err != nil {
			return nil, fmt.Errorf("creating worktree parent: %w", err)
		}

		if needsBranch {
			// Create branch + worktree in one step, without touching
			// the main working directory.
			if err := sdk.Git.WorktreeCreate(absWorktree, branch); err != nil {
				return nil, fmt.Errorf("creating worktree: %w", err)
			}
			if err := a.store.Tasks.Set(a.ctx, taskKey, author, tasks.SetOptions{
				Branch: &branch,
			}); err != nil {
				return nil, taskErr(err)
			}
		} else {
			if err := sdk.Git.WorktreeAdd(absWorktree, branch); err != nil {
				return nil, fmt.Errorf("creating worktree: %w", err)
			}
		}
	}

	// Resolve platform-specific behaviour for this agent.
	plat := assets.Agent.Platform(agent)

	// Write agent runtime settings (e.g. permissions, hooks) into
	// the working directory. Settings are templated with task-specific
	// values so hooks can reference the task ID and agent name.
	if settings := a.store.Agents.Settings(a.ctx, agent); settings != "" {
		templated := strings.ReplaceAll(settings, "{{.Agent}}", agent)
		templated = strings.ReplaceAll(templated, "{{.TaskID}}", taskKey)
		a.writeSettings(absWorktree, plat, templated)
	}

	// Build the context prompt.
	prompt := opts.Prompt
	if prompt == "" {
		llmdURL := ""
		if c, err := config.Load(); err == nil && c.Server.Addr != "" {
			llmdURL = "http://" + c.Server.Addr
		}
		prompt = a.store.Agents.BuildPrompt(a.ctx, cfg, agents.PromptData{
			Key:        t.Key,
			Title:      t.Title,
			Branch:     branch,
			AssignedTo: t.AssignedTo,
			Agent:      agent,
			URL:        llmdURL,
			SpecPath:   t.Path,
			OnSuccess:  opts.OnSuccess,
			OnFailure:  opts.OnFailure,
		})
	}

	// Build command args, replacing {{.Prompt}} placeholder.
	cmdArgs := make([]string, len(cfg.Args))
	for i, arg := range cfg.Args {
		cmdArgs[i] = strings.ReplaceAll(arg, "{{.Prompt}}", prompt)
	}

	// Apply budget.
	budget := cfg.MaxBudget
	if opts.MaxBudget > 0 {
		budget = opts.MaxBudget
	}
	if budget > 0 {
		cmdArgs = append(cmdArgs, plat.BudgetArgs(budget)...)
	}

	// Resolve the llmd binary path for the wrapper script.
	llmdPath, err := os.Executable()
	if err != nil {
		llmdPath, _ = exec.LookPath("llmd")
	}

	// Resolve the HTTP API URL from config. The wrapper uses HTTP
	// as primary transport (faster, no process spawn per call) and
	// falls back to the CLI binary.
	llmdURL := ""
	if cfg, err := config.Load(); err == nil && cfg.Server.Addr != "" {
		llmdURL = "http://" + cfg.Server.Addr
	}

	// Generate the wrapper script that runs the agent and handles
	// task lifecycle (moving the task on completion/failure).
	wrapperPath, err := agents.GenerateWrapper(absWorktree, agents.WrapperData{
		TaskID:    taskKey,
		Agent:     agent,
		Role:      cfg.Role,
		LLMD:      llmdPath,
		URL:       llmdURL,
		Worktree:  absWorktree,
		Command:   cmdPath,
		Args:      shellJoin(cmdArgs),
		OnSuccess: opts.OnSuccess,
		OnFailure: opts.OnFailure,
	})
	if err != nil {
		if cfg.Role != "auditor" {
			sdk.Git.WorktreeRemove(absWorktree)
		}
		return nil, fmt.Errorf("generating wrapper: %w", err)
	}

	// Start the wrapper script instead of the agent directly.
	cmd := exec.Command("/bin/bash", wrapperPath)
	cmd.Dir = absWorktree
	cmd.Env = os.Environ()

	// Send output to a log file so we can debug if needed.
	logPath := filepath.Join(absWorktree, ".llmd-agent.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		slog.Warn("creating agent log file", "path", logPath, "error", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		if cfg.Role != "auditor" {
			if rmErr := sdk.Git.WorktreeRemove(absWorktree); rmErr != nil {
				slog.Warn("cleaning up worktree after spawn failure", "path", absWorktree, "error", rmErr)
			}
		}
		return nil, fmt.Errorf("starting agent process: %w", err)
	}

	// Record the run.
	r, err := a.store.Agents.Record(a.ctx, agents.RecordOpts{
		TaskKey:  taskKey,
		Agent:    agent,
		Branch:   branch,
		Worktree: absWorktree,
		PID:      cmd.Process.Pid,
		Author:   author,
	})
	if err != nil {
		cmd.Process.Kill()
		if cfg.Role != "auditor" {
			if rmErr := sdk.Git.WorktreeRemove(absWorktree); rmErr != nil {
				slog.Warn("cleaning up worktree after record failure", "path", absWorktree, "error", rmErr)
			}
		}
		return nil, agentErr(err)
	}

	// Non-auditors move to in-progress when their agent starts, if
	// the column exists. Pipeline-driven boards may not have an
	// in-progress column - the task stays in the pipeline step column.
	if cfg.Role != "auditor" {
		cols, _ := a.store.Tasks.Columns(a.ctx)
		if slices.Contains(cols, "in-progress") {
			if err := a.store.Tasks.Move(a.ctx, taskKey, "in-progress", author); err != nil {
				slog.Warn("moving task to in-progress after spawn", "key", taskKey, "error", err)
			}
		}
	}

	return runToSDK(r), nil
}

// Run returns the most recent run for a task.
func (a *agentAPI) Run(taskKey string) (*sdk.AgentRun, error) {
	r, err := a.store.Agents.RunByTask(a.ctx, taskKey)
	if err != nil {
		return nil, agentErr(err)
	}
	return runToSDK(r), nil
}

// Runs returns agent runs matching the filter.
func (a *agentAPI) Runs(opts sdk.RunListOpts) ([]*sdk.AgentRun, error) {
	rr, err := a.store.Agents.List(a.ctx, agents.ListOpts{
		Status:  opts.Status,
		TaskKey: opts.TaskKey,
		Agent:   opts.Agent,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*sdk.AgentRun, len(rr))
	for i, r := range rr {
		out[i] = runToSDK(r)
	}
	return out, nil
}

// Prompt returns the prompt template for an agent/role combination.
func (a *agentAPI) Prompt(name, role string) (string, string, error) {
	content, path, err := a.store.Agents.Prompt(a.ctx, name, role)
	if err != nil {
		return "", "", agentErr(err)
	}
	return content, path, nil
}

// WritePrompt writes a prompt template for an agent/role.
func (a *agentAPI) WritePrompt(name, role, content, author string) error {
	return a.store.Agents.WritePrompt(a.ctx, name, role, content, author)
}

// Complete records an agent run as finished. It looks up the run,
// extracts stats from the output log via the Platform, and updates
// the database. The worktree is left intact for subsequent pipeline
// steps (testing, auditing) and cleaned up by task finish.
func (a *agentAPI) Complete(taskKey string, exitCode int) error {
	r, err := a.store.Agents.RunByTask(a.ctx, taskKey)
	if err != nil {
		return agentErr(err)
	}

	// Extract stats from the agent's output log.
	plat := assets.Agent.Platform(r.Agent)
	logPath := filepath.Join(r.Worktree, ".llmd-agent.log")
	stats, err := plat.Stats(logPath)
	if err != nil {
		slog.Debug("extracting agent stats", "task", taskKey, "error", err)
	}

	opts := agents.CompleteOpts{ExitCode: exitCode}
	if stats != nil {
		opts.MonetaryCost = stats.MonetaryCost
		opts.InputTokens = stats.InputTokens
		opts.OutputTokens = stats.OutputTokens
		opts.Model = stats.Model
	}

	return agentErr(a.store.Agents.Complete(a.ctx, taskKey, opts))
}

// Stop terminates a running agent process and cleans up its worktree.
func (a *agentAPI) Stop(taskKey, author string) error {
	r, err := a.store.Agents.RunByTask(a.ctx, taskKey)
	if err != nil {
		return agentErr(err)
	}
	if r.Status != "running" {
		return fmt.Errorf("agent run %s is not running", r.Key)
	}

	// Kill the process.
	if r.PID > 0 {
		proc, err := os.FindProcess(r.PID)
		if err == nil {
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				slog.Debug("sending SIGTERM to agent", "pid", r.PID, "error", err)
				proc.Kill()
			}
		}
	}

	// Mark as stopped.
	if _, err := a.store.Agents.MarkStopped(a.ctx, taskKey, author); err != nil {
		return agentErr(err)
	}

	// Clean up worktree.
	if r.Worktree != "" {
		if err := sdk.Git.WorktreeRemove(r.Worktree); err != nil {
			slog.Warn("removing worktree after stop", "path", r.Worktree, "error", err)
		}
	}

	return nil
}

// writeSettings writes agent runtime settings into the worktree at
// the path determined by the platform.
func (a *agentAPI) writeSettings(worktree string, plat assets.Platform, content string) {
	rel := plat.SettingsPath()
	if rel == "" {
		return
	}
	path := filepath.Join(worktree, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("creating agent settings directory", "path", filepath.Dir(path), "error", err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		slog.Warn("writing agent settings", "path", path, "error", err)
	}
}

// shellJoin combines args into a shell-safe string. Arguments
// containing spaces or special characters are single-quoted.
func shellJoin(args []string) string {
	var parts []string
	for _, a := range args {
		if strings.ContainsAny(a, " \t\n\"'\\$`!#&|;(){}[]<>?*~") {
			parts = append(parts, "'"+strings.ReplaceAll(a, "'", "'\\''")+"'")
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
