package host

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/internal/llmd/audits"
	"github.com/jpl-au/llmd/internal/llmd/rules"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/internal/telemetry"
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
		SessionID:    r.SessionID,
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

	// Resolve transitions from the rule if not explicitly provided.
	if opts.OnSuccess == "" || opts.OnFailure == "" {
		rs, err := rules.Load(a.store.Dir(), "default")
		if err == nil {
			cr := rs.Column(t.Status)
			if opts.OnSuccess == "" {
				opts.OnSuccess = cr.Success
			}
			if opts.OnFailure == "" {
				opts.OnFailure = cr.Failure
			}
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
		// Create or reuse worktree. Multiple agents may work on the
		// same task (developer then tester), sharing the worktree.
		worktreeDir := filepath.Join(".llmd", "worktrees", taskKey)
		absWorktree, err = filepath.Abs(worktreeDir)
		if err != nil {
			return nil, fmt.Errorf("resolving worktree path: %w", err)
		}

		// Reuse existing worktree from a previous pipeline step.
		if _, statErr := os.Stat(absWorktree); statErr == nil {
			slog.Debug("reusing existing worktree", "path", absWorktree)
		} else {
			if err := os.MkdirAll(filepath.Dir(absWorktree), 0755); err != nil {
				return nil, fmt.Errorf("creating worktree parent: %w", err)
			}
			if needsBranch {
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

	// Resolve the HTTP API URL. Only use it if the server is
	// actually reachable - a configured but stopped server should
	// not cause agents to receive HTTP-based prompts they can't use.
	llmdURL := ""
	if c, err := config.Load(); err == nil && c.Server.Addr != "" {
		candidate := "http://" + c.Server.Addr
		if resp, err := http.Get(candidate + "/version"); err == nil {
			resp.Body.Close()
			llmdURL = candidate
		}
	}

	// Resume is requested either explicitly via SpawnOpts (set by the
	// pipeline handler from the column rule, or by CLI --resume) or
	// persistently via the task's "resume" flag. Either source being
	// true triggers an attempt to pick up the previous agent's
	// conversation context, avoiding a cold start where the agent
	// has no memory of its prior work. The attempt is guarded by
	// several conditions: the previous run must exist, must have been
	// by the same agent (a gemini session ID is meaningless to claude),
	// must have produced a session ID, and the platform must support
	// resume. If any condition fails, we fall through to a fresh prompt.
	//
	// When resuming, the agent still needs direction about what to
	// fix. We build a lightweight prompt from audit entries created
	// since the previous run started (these are typically the
	// auditor's rejection feedback). This prompt is passed alongside
	// the session resume so the agent gets its old context plus the
	// new instructions.
	var cmdArgs []string
	resumed := false
	if opts.Resume || hasFlag(t.Flags, "resume") {
		if prev, prevErr := a.store.Agents.RunByTask(a.ctx, taskKey); prevErr == nil {
			if prev.Agent == agent && prev.SessionID != "" {
				resumePrompt := a.buildResumePrompt(taskKey, prev.StartedAt)
				if resumeArgs := plat.ResumeArgs(prev.SessionID, resumePrompt); resumeArgs != nil {
					cmdArgs = resumeArgs
					resumed = true
					slog.Debug("resuming previous session", "task", taskKey, "session", prev.SessionID)
				}
			}
		}
	}

	if !resumed {
		prompt := opts.Prompt
		if prompt == "" {
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

		cmdArgs = make([]string, len(cfg.Args))
		for i, arg := range cfg.Args {
			cmdArgs[i] = strings.ReplaceAll(arg, "{{.Prompt}}", prompt)
		}
	}

	// Apply budget.
	budget := cfg.MaxBudget
	if opts.MaxBudget > 0 {
		budget = opts.MaxBudget
	}
	if budget > 0 {
		cmdArgs = append(cmdArgs, plat.BudgetArgs(budget)...)
	}

	// Write the resolved run config so agent run can read it.
	if err := agents.WriteRunConfig(absWorktree, agents.RunConfig{
		TaskKey:   taskKey,
		Agent:     agent,
		Role:      cfg.Role,
		Command:   cmdPath,
		Args:      cmdArgs,
		OnSuccess: opts.OnSuccess,
		OnFailure: opts.OnFailure,
	}); err != nil {
		if cfg.Role != "auditor" {
			sdk.Git.WorktreeRemove(absWorktree)
		}
		return nil, fmt.Errorf("writing run config: %w", err)
	}

	// Start llmd agent run as a detached process. It handles the
	// full agent lifecycle: starting the agent, waiting for exit,
	// recording completion, and moving the task.
	llmdPath, err := os.Executable()
	if err != nil {
		llmdPath, _ = exec.LookPath("llmd")
	}

	cmd := exec.Command(llmdPath, "--author", agent, "agent", "run", taskKey, "--worktree", absWorktree)
	cmd.Dir, _ = filepath.Abs(".")
	cmd.Env = os.Environ()
	detach(cmd)

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

	// Capture the raw JSON response for diagnostic telemetry. This
	// preserves the full LLM output (token counts, model info, cost,
	// session ID, and any tool-specific fields) so it can be analysed
	// later without re-running the agent. Only emitted when the
	// binary is compiled with the telemetry build tag.
	if rawJSON, jsonErr := assets.LastJSON(logPath); jsonErr == nil && rawJSON != "" {
		telemetry.EmitAgent(telemetry.AgentEntry{
			RunKey:  r.Key,
			TaskKey: taskKey,
			Agent:   r.Agent,
			RawJSON: rawJSON,
		})
	}

	opts := agents.CompleteOpts{ExitCode: exitCode}
	if stats != nil {
		opts.MonetaryCost = stats.MonetaryCost
		opts.InputTokens = stats.InputTokens
		opts.OutputTokens = stats.OutputTokens
		opts.Model = stats.Model
		opts.SessionID = stats.SessionID
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

	// Gracefully terminate the process. On Unix this sends SIGTERM; on
	// Windows it sends CTRL_BREAK_EVENT to the process group. If the
	// graceful signal fails, fall back to a hard kill.
	if r.PID > 0 {
		proc, err := os.FindProcess(r.PID)
		if err == nil {
			if err := terminate(proc); err != nil {
				slog.Debug("graceful termination failed, killing", "pid", r.PID, "error", err)
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

// buildResumePrompt assembles a lightweight prompt for a resumed
// session from audit entries created since the previous run started.
// These entries are typically the auditor's rejection feedback
// explaining what needs fixing. The resumed agent already has the
// full task context from its previous session, so this prompt
// focuses only on what changed since then. Returns a generic
// instruction if no audit feedback is found, because the agent
// still needs a prompt to trigger non-interactive execution.
func (a *agentAPI) buildResumePrompt(taskKey string, sinceMS int64) string {
	audits, err := a.store.Audits.List(a.ctx, audits.ListOptions{
		Target:  taskKey,
		SinceMS: sinceMS,
	})
	if err != nil {
		slog.Debug("reading audits for resume prompt", "task", taskKey, "error", err)
	}

	if len(audits) == 0 {
		return fmt.Sprintf("Resume work on task %s. Review any recent feedback and address outstanding issues.", taskKey)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Your previous work on task %s was reviewed. Address the following feedback:\n\n", taskKey)
	for _, aud := range audits {
		if aud.Content == "" {
			continue
		}
		fmt.Fprintf(&b, "--- %s (%s):\n%s\n\n", aud.Author, aud.Status, aud.Content)
	}
	return b.String()
}

func hasFlag(flags, flag string) bool {
	for f := range strings.SplitSeq(flags, ",") {
		if strings.TrimSpace(f) == flag {
			return true
		}
	}
	return false
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
