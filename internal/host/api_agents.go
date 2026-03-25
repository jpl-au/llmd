package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	texttemplate "text/template"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/internal/llmd/audits"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/pkg/model/task"
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
		Key:       r.Key,
		TaskKey:   r.TaskKey,
		Agent:     r.Agent,
		Branch:    r.Branch,
		Worktree:  r.Worktree,
		Status:    r.Status,
		PID:       r.PID,
		ExitCode:  r.ExitCode,
		Author:    r.Author,
		StartedAt: r.StartedAt,
		StoppedAt: r.StoppedAt,
	}
}

// Register adds or updates an agent configuration.
func (a *agentAPI) Register(cfg sdk.AgentConfig, author string) error {
	return a.store.Agents.Register(a.ctx, agents.Config{
		Name:      cfg.Name,
		Command:   cfg.Command,
		Args:      cfg.Args,
		Role:      cfg.Role,
		MaxBudget: cfg.MaxBudget,
	}, author)
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
	return &sdk.AgentConfig{
		Name:      cfg.Name,
		Command:   cfg.Command,
		Args:      cfg.Args,
		Role:      cfg.Role,
		MaxBudget: cfg.MaxBudget,
	}, nil
}

// Agents returns all registered agent configurations.
func (a *agentAPI) Agents() ([]sdk.AgentConfig, error) {
	cfgs, err := a.store.Agents.Agents(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.AgentConfig, len(cfgs))
	for i, c := range cfgs {
		out[i] = sdk.AgentConfig{
			Name:      c.Name,
			Command:   c.Command,
			Args:      c.Args,
			Role:      c.Role,
			MaxBudget: c.MaxBudget,
		}
	}
	return out, nil
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

	// Create worktree (and branch if needed).
	worktreeDir := filepath.Join(".llmd", "worktrees", taskKey)
	absWorktree, err := filepath.Abs(worktreeDir)
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

	// Write agent runtime settings (e.g. permissions, hooks) into
	// the worktree. Settings are templated with task-specific values
	// so hooks can reference the task ID and agent name.
	if settings := a.store.Agents.Settings(a.ctx, agent); settings != "" {
		templated := strings.ReplaceAll(settings, "{{.Agent}}", agent)
		templated = strings.ReplaceAll(templated, "{{.TaskID}}", taskKey)
		a.writeWorktreeSettings(absWorktree, agent, templated)
	}

	// Build the context prompt.
	prompt := opts.Prompt
	if prompt == "" {
		prompt = a.buildContext(t, cfg)
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
		cmdArgs = appendBudgetFlag(cfg.Command, cmdArgs, budget)
	}

	// Resolve the llmd binary path for the wrapper script.
	llmdPath, err := os.Executable()
	if err != nil {
		llmdPath, _ = exec.LookPath("llmd")
	}

	// Generate the wrapper script that runs the agent and handles
	// task lifecycle (moving the task on completion/failure).
	wrapperPath, err := agents.GenerateWrapper(absWorktree, agents.WrapperData{
		TaskID:   taskKey,
		Agent:    agent,
		LLMD:     llmdPath,
		Worktree: absWorktree,
		Command:  cmdPath,
		Args:     shellJoin(cmdArgs),
	})
	if err != nil {
		sdk.Git.WorktreeRemove(absWorktree)
		return nil, fmt.Errorf("generating wrapper: %w", err)
	}

	// Start the wrapper script instead of the agent directly.
	cmd := exec.Command("/bin/bash", wrapperPath)
	cmd.Dir = absWorktree
	cmd.Env = append(os.Environ(),
		"LLMD_TASK_ID="+taskKey,
		"LLMD_AGENT="+agent,
	)

	// Send output to a log file so we can debug if needed.
	logPath := filepath.Join(absWorktree, ".llmd-agent.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		slog.Warn("creating agent log file", "path", logPath, "error", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		// Clean up worktree on spawn failure.
		if rmErr := sdk.Git.WorktreeRemove(absWorktree); rmErr != nil {
			slog.Warn("cleaning up worktree after spawn failure", "path", absWorktree, "error", rmErr)
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
		// Kill the process if we can't record it.
		cmd.Process.Kill()
		if rmErr := sdk.Git.WorktreeRemove(absWorktree); rmErr != nil {
			slog.Warn("cleaning up worktree after record failure", "path", absWorktree, "error", rmErr)
		}
		return nil, agentErr(err)
	}

	// Move the task to in-progress.
	if err := a.store.Tasks.Move(a.ctx, taskKey, "in-progress", author); err != nil {
		slog.Warn("moving task to in-progress after spawn", "key", taskKey, "error", err)
	}

	// Wait for the process in a goroutine so we can record completion.
	go a.waitForProcess(cmd, taskKey, absWorktree, logFile)

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

// Stop terminates a running agent and cleans up its worktree.
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

// waitForProcess waits for the agent subprocess to exit and records the
// result. Runs in a goroutine started by Spawn.
func (a *agentAPI) waitForProcess(cmd *exec.Cmd, taskKey, worktree string, logFile *os.File) {
	if logFile != nil {
		defer logFile.Close()
	}
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	ctx := context.Background()
	if err := a.store.Agents.Complete(ctx, taskKey, exitCode); err != nil {
		slog.Warn("recording agent completion", "task", taskKey, "error", err)
	}

	// Clean up worktree after completion.
	if worktree != "" {
		if err := sdk.Git.WorktreeRemove(worktree); err != nil {
			slog.Debug("removing worktree after completion", "path", worktree, "error", err)
		}
	}
}

// promptData is the data passed to prompt templates during execution.
type promptData struct {
	Key        string
	Title      string
	Branch     string
	AssignedTo string
	Agent      string
	Spec       string
	Diff       string
	LinkedDocs []linkedDoc
	Audits     []auditEntry
}

type linkedDoc struct {
	Path    string
	Content string
}

type auditEntry struct {
	Author  string
	Status  string
	Content string
}

// buildContext assembles a prompt by executing the stored prompt
// template for the agent/role combination. Falls back through:
// agents/<name>/<role> → agents/default/<role> → built-in default.
func (a *agentAPI) buildContext(t *task.Task, cfg *agents.Config) string {
	role := cfg.Role
	if role == "" {
		role = "developer"
	}

	// Gather template data.
	data := a.gatherPromptData(t, cfg)

	// Read the prompt template.
	tmplContent, _, err := a.store.Agents.Prompt(a.ctx, cfg.Name, role)
	if err != nil {
		// Fall back to built-in default.
		slog.Debug("no stored prompt template, using built-in", "agent", cfg.Name, "role", role)
		tmplContent = agents.DefaultTemplate(role)
		if tmplContent == "" {
			tmplContent = agents.DefaultTemplate("developer")
		}
	}

	// Execute the template.
	tmpl, err := texttemplate.New("prompt").Parse(tmplContent)
	if err != nil {
		slog.Warn("parsing prompt template", "agent", cfg.Name, "role", role, "error", err)
		return fmt.Sprintf("Task %s: %s\nBranch: %s\n\n%s", t.Key, t.Title, t.Branch, data.Spec)
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		slog.Warn("executing prompt template", "agent", cfg.Name, "role", role, "error", err)
		return fmt.Sprintf("Task %s: %s\nBranch: %s\n\n%s", t.Key, t.Title, t.Branch, data.Spec)
	}

	return b.String()
}

// gatherPromptData reads all the context data needed by prompt templates.
func (a *agentAPI) gatherPromptData(t *task.Task, cfg *agents.Config) promptData {
	data := promptData{
		Key:        t.Key,
		Title:      t.Title,
		Branch:     t.Branch,
		AssignedTo: t.AssignedTo,
		Agent:      cfg.Name,
	}

	// Read the spec document.
	if t.Path != "" {
		doc, err := a.store.Documents.Read(a.ctx, t.Path)
		if err == nil {
			data.Spec = doc.Content
		} else {
			slog.Debug("reading task spec for agent context", "path", t.Path, "error", err)
		}
	}

	// Read linked documents.
	if t.Path != "" {
		ll, err := a.store.Links.List(a.ctx, t.Path, links.Options{Direction: links.Outgoing})
		if err == nil {
			for _, lk := range ll {
				doc, err := a.store.Documents.Read(a.ctx, lk.Value.To)
				if err == nil {
					data.LinkedDocs = append(data.LinkedDocs, linkedDoc{
						Path:    lk.Value.To,
						Content: doc.Content,
					})
				}
			}
		}
	}

	// Read audit history.
	auditList, err := a.store.Audits.List(a.ctx, audits.ListOptions{Target: t.Key})
	if err == nil {
		for _, au := range auditList {
			data.Audits = append(data.Audits, auditEntry{
				Author:  au.Author,
				Status:  au.Status,
				Content: au.Content,
			})
		}
	}

	// Git diff (useful for auditor role).
	if t.Branch != "" && sdk.Git.Available() == nil {
		base, err := sdk.Git.DefaultBranch()
		if err == nil {
			diff, err := sdk.Git.Diff(base, t.Branch, sdk.DiffOpts{})
			if err == nil {
				data.Diff = diff
			}
		}
	}

	return data
}

// writeWorktreeSettings writes agent-specific settings into the
// worktree. The file location depends on the agent platform.
func (a *agentAPI) writeWorktreeSettings(worktree, agent, content string) {
	// Claude Code reads project settings from .claude/settings.json
	if strings.Contains(agent, "claude") {
		dir := filepath.Join(worktree, ".claude")
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Warn("creating agent settings directory", "path", dir, "error", err)
			return
		}
		path := filepath.Join(dir, "settings.json")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			slog.Warn("writing agent settings", "path", path, "error", err)
		}
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

// appendBudgetFlag adds a budget flag appropriate for the agent
// platform. Currently only supports Claude Code's --max-budget-usd.
func appendBudgetFlag(command string, args []string, budget float64) []string {
	if strings.Contains(command, "claude") {
		return append(args, fmt.Sprintf("--max-budget-usd=%.2f", budget))
	}
	return args
}
