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
	"time"

	"github.com/jpl-au/llmd/ai/harness"
	"github.com/jpl-au/llmd/assets"
	"github.com/jpl-au/llmd/assets/platform"
	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/agents"
	"github.com/jpl-au/llmd/engine/audits"
	"github.com/jpl-au/llmd/engine/rules"
	"github.com/jpl-au/llmd/engine/tasks"
	"github.com/jpl-au/llmd/errs"
	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/telemetry"
)

// agentAPI implements [sdk.AgentStore] by delegating data operations to
// the internal agents package and handling orchestration (worktree
// creation, context assembly, subprocess spawning) in the bridge layer.
// This follows the same pattern as taskAPI handling git operations.
type agentAPI struct {
	ctx      context.Context
	store    *engine.Store
	git      sdk.GitStore
	worktree bool   // agents get their own worktree unless SpawnOpts says otherwise
	harness  string // agent.harness: the harness this computer runs; empty leaves it to the store
}

func newAgentAPI(store *engine.Store, git sdk.GitStore, worktree bool, harness string, ctx context.Context) *agentAPI {
	return &agentAPI{ctx: ctx, store: store, git: git, worktree: worktree, harness: harness}
}

// harnessKey is the key of the setting that names the harness an agent runs under.
func harnessKey(agent string) string {
	return "agent." + agent + ".harness"
}

// Harness returns the harness an agent runs under on this computer.
func (a *agentAPI) Harness(name string) (string, error) {
	if _, err := a.store.Agents.Agent(a.ctx, name); err != nil {
		return "", agentErr(err)
	}
	if a.harness != "" {
		return a.harness, nil
	}
	value, ok, err := a.store.Settings.Value(a.ctx, harnessKey(name))
	if err != nil {
		return "", err
	}
	if !ok {
		return harness.Default, nil
	}
	return value, nil
}

// SetHarness names the harness an agent runs under, in the store.
func (a *agentAPI) SetHarness(name, h, author string) error {
	if _, err := a.store.Agents.Agent(a.ctx, name); err != nil {
		return agentErr(err)
	}
	if _, ok := harness.Lookup(h); !ok {
		return unknownHarness(h)
	}
	return a.store.Settings.Set(a.ctx, harnessKey(name), h, author)
}

// unknownHarness is the error for a harness this build of llmd does not have.
func unknownHarness(name string) error {
	return sdk.Errorf(sdk.ErrNotFound, "this build of %s has no harness %s. Harnesses: %s.", sdk.Program(), name, strings.Join(harness.Names(), ", "))
}

func agentErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, agents.ErrNotFound):
		return errs.Mark(sdk.ErrNotFound, err)
	case errors.Is(err, agents.ErrRunNotFound):
		return errs.Mark(sdk.ErrNotFound, err)
	case errors.Is(err, agents.ErrRunning):
		return errs.Mark(sdk.ErrAgentRunning, err)
	case errors.Is(err, agents.ErrNotRunning):
		return errs.Mark(sdk.ErrAgentNotRunning, err)
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
func (a *agentAPI) Register(cfg sdk.AgentConfig) error {
	return a.store.Agents.Register(a.ctx, cfg)
}

// Remove deletes an agent configuration by name.
func (a *agentAPI) Remove(name string) error {
	return agentErr(a.store.Agents.Remove(a.ctx, name))
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

// runsInPlace reports whether an agent runs in the main checkout rather
// than its own git worktree. Auditors review existing work, so they
// always run in the main checkout and read the developer's diff via
// task commands. Other roles run there when worktrees are turned off:
// override sets it for one spawn, and a nil override uses worktree,
// the agent.worktree configuration.
func runsInPlace(role string, worktree bool, override *bool) bool {
	if role == "auditor" {
		return true
	}
	if override != nil {
		return !*override
	}
	return !worktree
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
		return nil, sdk.Errorf(sdk.ErrNotReady, "%s is waiting on other tasks. Run %s task chain %s to see them.", taskKey, sdk.Program(), taskKey)
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

	// A task runs one agent at a time. Refuse here, before anything
	// is created: a refusal after the worktree and the process existed
	// once removed the running agent's worktree. Record checks again,
	// atomically, for two spawns racing past this point.
	if prev, err := a.store.Agents.RunByTask(a.ctx, taskKey); err == nil && prev.Status == "running" {
		return nil, agentErr(agents.Running(taskKey))
	}

	// The harness must be in this build, since the run starts this same binary.
	harnessName, err := a.Harness(agent)
	if err != nil {
		return nil, err
	}
	if _, ok := harness.Lookup(harnessName); !ok {
		return nil, unknownHarness(harnessName)
	}

	// Resolve the command binary.
	cmdPath, err := exec.LookPath(cfg.Command)
	if err != nil {
		return nil, fmt.Errorf("agent command %q not found: %w", cfg.Command, err)
	}

	// Ensure git is available - worktrees are required for agent isolation.
	if err := a.git.Available(); err != nil {
		return nil, fmt.Errorf("git required for agent spawn: %w", err)
	}

	// Agents reach llmd through the HTTP server: their prompts and
	// hooks call it, so they work from a worktree that has no store.
	// Refuse before anything is created when it is not running.
	llmdURL, auth, err := serverURL()
	if err != nil {
		return nil, err
	}

	inPlace := runsInPlace(cfg.Role, a.worktree, opts.Worktree)

	// Determine the branch name. An agent in the main checkout works
	// on whatever branch is checked out there.
	branch := t.Branch
	needsBranch := branch == "" && !inPlace
	if needsBranch {
		branch = "task/" + branchSlug(t.Title)
	}
	if branch == "" {
		if b, err := a.git.Branch(); err == nil {
			branch = b
		}
	}

	// created is whether this spawn made the worktree: only then may a
	// failure below remove it. A reused worktree belongs to an earlier
	// pipeline step, or to an agent that is still running.
	var absWorktree string
	created := false
	if inPlace {
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
				if err := a.git.WorktreeCreate(absWorktree, branch); err != nil {
					return nil, fmt.Errorf("creating worktree: %w", err)
				}
				created = true
				if err := a.store.Tasks.Set(a.ctx, taskKey, author, tasks.SetOptions{
					Branch: &branch,
				}); err != nil {
					return nil, taskErr(err)
				}
			} else {
				if err := a.git.WorktreeAdd(absWorktree, branch); err != nil {
					return nil, fmt.Errorf("creating worktree: %w", err)
				}
				created = true
			}
		}
	}

	// runDir holds the run config, settings and log. A worktree is
	// the agent's own, so they go in it; in the main checkout they go
	// under .llmd/runs/<task>/, so two agents there keep theirs apart.
	runDir := absWorktree
	if inPlace {
		runDir, err = a.runDir(taskKey)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(runDir, 0755); err != nil {
			return nil, fmt.Errorf("creating run directory: %w", err)
		}
	}

	// Resolve platform-specific behaviour for this agent.
	plat := assets.Agent.Platform(agent)

	// Write agent runtime settings (e.g. permissions, hooks).
	// Settings are templated with task-specific values so hooks can
	// reference the task ID and agent name. In a worktree they go
	// where the platform reads them. The main checkout's settings file
	// is the person's own, so there they go under .llmd/runs/ and are
	// passed to the agent by flag from the run directory.
	var settingsPath string
	if settings := a.store.Agents.Settings(a.ctx, agent); settings != "" && plat.SettingsPath() != "" {
		templated := strings.ReplaceAll(settings, "{{.Agent}}", agent)
		templated = strings.ReplaceAll(templated, "{{.TaskID}}", taskKey)
		templated = strings.ReplaceAll(templated, "{{.URL}}", llmdURL)
		templated = strings.ReplaceAll(templated, "{{.Auth}}", auth)
		if inPlace {
			path := filepath.Join(runDir, "settings.json")
			if writeSettings(path, templated) {
				settingsPath = path
			}
		} else {
			writeSettings(filepath.Join(absWorktree, plat.SettingsPath()), templated)
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
	// and must have produced a session ID. The harness carries the
	// session on where it can, and otherwise runs the fresh prompt.
	//
	// When resuming, the agent still needs direction about what to
	// fix. We build a lightweight prompt from audit entries created
	// since the previous run started (these are typically the
	// auditor's rejection feedback). This prompt is passed alongside
	// the session resume so the agent gets its old context plus the
	// new instructions.
	var resume *sdk.Resume
	if opts.Resume || hasFlag(t.Flags, "resume") {
		if prev, prevErr := a.store.Agents.RunByTask(a.ctx, taskKey); prevErr == nil {
			if prev.Agent == agent && prev.SessionID != "" {
				resume = &sdk.Resume{Session: prev.SessionID, Prompt: a.buildResumePrompt(taskKey, prev.StartedAt)}
				slog.Debug("resuming previous session", "task", taskKey, "session", prev.SessionID)
			}
		}
	}

	prompt := opts.Prompt
	if prompt == "" {
		prompt = a.store.Agents.BuildPrompt(a.ctx, cfg, agents.PromptData{
			Key:        t.Key,
			Title:      t.Title,
			Branch:     branch,
			AssignedTo: t.AssignedTo,
			Agent:      agent,
			URL:        llmdURL,
			Auth:       auth,
			SpecPath:   t.Path,
			OnSuccess:  opts.OnSuccess,
			OnFailure:  opts.OnFailure,
		})
	}

	budget := cfg.MaxBudget
	if opts.MaxBudget > 0 {
		budget = opts.MaxBudget
	}

	// Write the resolved run config so agent run can read it.
	if err := agents.WriteRunConfig(runDir, agents.RunConfig{
		TaskKey:   taskKey,
		Role:      cfg.Role,
		OnSuccess: opts.OnSuccess,
		OnFailure: opts.OnFailure,
		Harness:   harnessName,
		Spec: sdk.RunSpec{
			Agent:    agent,
			Command:  cmdPath,
			Args:     cfg.Args,
			Prompt:   prompt,
			Resume:   resume,
			Settings: settingsPath,
			Budget:   budget,
			Dir:      absWorktree,
			Log:      filepath.Join(runDir, agents.LogFile),
		},
	}); err != nil {
		if created {
			if rmErr := a.git.WorktreeRemove(absWorktree); rmErr != nil {
				slog.Warn("cleaning up worktree after config failure", "path", absWorktree, "error", rmErr)
			}
		}
		return nil, fmt.Errorf("writing run config: %w", err)
	}

	// Start llmd agent run as a detached process. It handles the
	// full agent lifecycle: starting the agent, waiting for exit,
	// recording completion, and moving the task.
	llmdPath, err := os.Executable()
	if err != nil {
		llmdPath, err = exec.LookPath("llmd")
		if err != nil {
			return nil, fmt.Errorf("finding llmd binary: %w", err)
		}
	}

	cmd := exec.Command(llmdPath, "--author", agent, "agent", "run", taskKey, "--worktree", runDir)
	cmd.Dir, err = filepath.Abs(".")
	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}
	cmd.Env = os.Environ()
	detach(cmd)

	if err := cmd.Start(); err != nil {
		if created {
			if rmErr := a.git.WorktreeRemove(absWorktree); rmErr != nil {
				slog.Warn("cleaning up worktree after spawn failure", "path", absWorktree, "error", rmErr)
			}
		}
		return nil, fmt.Errorf("starting agent process: %w", err)
	}

	// Record the run. A run in the main checkout has no worktree, so
	// task finish and agent stop leave the checkout alone.
	recorded := absWorktree
	if inPlace {
		recorded = ""
	}
	r, err := a.store.Agents.Record(a.ctx, agents.RecordOpts{
		TaskKey:  taskKey,
		Agent:    agent,
		Branch:   branch,
		Worktree: recorded,
		PID:      cmd.Process.Pid,
		Author:   author,
	})
	if err != nil {
		if killErr := cmd.Process.Kill(); killErr != nil {
			slog.Warn("killing agent after record failure", "pid", cmd.Process.Pid, "error", killErr)
		}
		if created {
			if rmErr := a.git.WorktreeRemove(absWorktree); rmErr != nil {
				slog.Warn("cleaning up worktree after record failure", "path", absWorktree, "error", rmErr)
			}
		}
		return nil, agentErr(err)
	}

	// Non-auditors take the task: the agent becomes the assignee and
	// the task moves to in-progress, if the column exists.
	// Pipeline-driven boards may not have an in-progress column - the
	// task stays in the pipeline step column.
	if cfg.Role != "auditor" {
		if err := a.store.Tasks.Set(a.ctx, taskKey, author, tasks.SetOptions{AssignedTo: &agent}); err != nil {
			slog.Warn("assigning task to agent after spawn", "key", taskKey, "agent", agent, "error", err)
		}
		cols, colErr := a.store.Tasks.Columns(a.ctx)
		if colErr != nil {
			slog.Debug("loading columns after spawn", "error", colErr)
		}
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

// Complete records an agent run as finished, with the usage its
// harness reported. The worktree is left intact for subsequent
// pipeline steps (testing, auditing) and cleaned up by task finish.
func (a *agentAPI) Complete(taskKey string, result sdk.RunResult) error {
	r, err := a.store.Agents.RunByTask(a.ctx, taskKey)
	if err != nil {
		return agentErr(err)
	}

	dir := r.Worktree
	if dir == "" {
		if dir, err = a.runDir(taskKey); err != nil {
			return err
		}
	}
	logPath := filepath.Join(dir, agents.LogFile)

	// Capture the raw JSON response for diagnostic telemetry. This
	// preserves the full LLM output (token counts, model info, cost,
	// session ID, and any tool-specific fields) so it can be analysed
	// later without re-running the agent. Only emitted when the
	// binary is compiled with the telemetry build tag.
	if rawJSON, jsonErr := platform.LastJSON(logPath); jsonErr == nil && rawJSON != "" {
		telemetry.EmitAgent(telemetry.AgentEntry{
			RunKey:  r.Key,
			TaskKey: taskKey,
			Agent:   r.Agent,
			RawJSON: rawJSON,
		})
	}

	return agentErr(a.store.Agents.Complete(a.ctx, taskKey, agents.CompleteOpts{
		ExitCode:     result.ExitCode,
		MonetaryCost: result.Cost,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		Model:        result.Model,
		SessionID:    result.Session,
	}))
}

// Stop terminates a running agent process and cleans up its worktree.
func (a *agentAPI) Stop(taskKey, author string) error {
	r, err := a.store.Agents.RunByTask(a.ctx, taskKey)
	if err != nil {
		return agentErr(err)
	}
	if r.Status != "running" {
		return agentErr(errs.New(agents.ErrNotRunning, "agent run %s is %s, not running.", r.Key, r.Status))
	}

	// Gracefully terminate the process. On Unix this sends SIGTERM; on
	// Windows it sends CTRL_BREAK_EVENT to the process group. If the
	// graceful signal fails, fall back to a hard kill.
	if r.PID > 0 {
		proc, err := os.FindProcess(r.PID)
		if err == nil {
			if err := terminate(proc); err != nil {
				slog.Debug("graceful termination failed, killing", "pid", r.PID, "error", err)
				if killErr := proc.Kill(); killErr != nil {
					slog.Warn("killing agent", "pid", r.PID, "error", killErr)
				}
			}
		}
	}

	// Mark as stopped.
	if _, err := a.store.Agents.MarkStopped(a.ctx, taskKey, author); err != nil {
		return agentErr(err)
	}

	// Clean up worktree.
	if r.Worktree != "" {
		if err := a.git.WorktreeRemove(r.Worktree); err != nil {
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

// serverURL returns the URL of the running llmd serve, read from
// server.addr, or an error saying to run it. auth is what an agent's
// curl adds to send server.token: a leading space and the header, or
// empty when no token is set.
func serverURL() (url, auth string, err error) {
	addr := config.DefaultAddr
	c, err := config.Load()
	if err == nil && c.Server.Addr != "" {
		addr = c.Server.Addr
	}
	if c.Server.Token != "" {
		auth = " -H 'Authorization: Bearer " + c.Server.Token + "'"
	}
	url = "http://" + addr
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/version")
	if err != nil {
		return "", "", sdk.Errorf(sdk.ErrUnavailable, "unable to spawn agents. Please run %s serve.", sdk.Program())
	}
	resp.Body.Close()
	return url, auth, nil
}

// runDir returns the directory that holds the run files of an agent
// working in the main checkout.
func (a *agentAPI) runDir(taskKey string) (string, error) {
	dir, err := filepath.Abs(filepath.Join(a.store.Dir(), "runs", taskKey))
	if err != nil {
		return "", fmt.Errorf("resolving run directory: %w", err)
	}
	return dir, nil
}

// writeSettings writes agent runtime settings to path and reports
// whether it did. A failure is logged: the agent still runs, without
// its settings.
func writeSettings(path, content string) bool {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("creating agent settings directory", "path", filepath.Dir(path), "error", err)
		return false
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		slog.Warn("writing agent settings", "path", path, "error", err)
		return false
	}
	return true
}
