package host

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/engine/agents"
	"github.com/jpl-au/llmd/sdk"
)

// agentHost returns an isolated host and a context acting as alice.
func agentHost(t *testing.T) (*Host, sdk.Context) {
	t.Helper()
	h := TestSetup(t, TestIsolated)
	return h, h.Context(t.Context(), "alice")
}

// claudeLog is the final JSON line Claude Code prints; Complete reads
// it from <worktree>/.llmd-agent.log to record the run's stats.
const claudeLog = `thinking...
{"total_cost_usd":0.42,"session_id":"sess-1","usage":{"input_tokens":10,"output_tokens":20},"modelUsage":{"claude-test":{}}}
`

func TestAgentsRegister(t *testing.T) {
	tests := []struct {
		name    string
		cfg     sdk.AgentConfig
		wantErr bool
	}{
		{"full config", sdk.AgentConfig{Name: "claude-code", Command: "claude", Args: []string{"-p", "{{.Prompt}}"}, Role: "developer", MaxBudget: 2.5}, false},
		{"minimal config", sdk.AgentConfig{Name: "tiny", Command: "sh"}, false},
		{"missing name", sdk.AgentConfig{Command: "sh"}, true},
		{"missing command", sdk.AgentConfig{Name: "nocmd"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentHost(t)
			err := c.Agents.Register(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Register(%+v) error = %v, wantErr %v", tt.cfg, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			got, err := c.Agents.Agent(tt.cfg.Name)
			if err != nil {
				t.Fatalf("Agent(%q) error = %v", tt.cfg.Name, err)
			}
			if !reflect.DeepEqual(*got, tt.cfg) {
				t.Errorf("Agent(%q) = %+v, want %+v", tt.cfg.Name, *got, tt.cfg)
			}
			// Registration seeds the prompt templates for the agent
			// and the default fallback.
			for _, name := range []string{tt.cfg.Name, "default"} {
				path := filepath.Join(h.Dir(), ".llmd", "agents", name, "developer.md")
				if _, err := os.Stat(path); err != nil {
					t.Errorf("Register(%q) did not seed %s: %v", tt.cfg.Name, path, err)
				}
			}
		})
	}
}

func TestAgentsAgents(t *testing.T) {
	tests := []struct {
		name     string
		register []string
		want     []string
	}{
		{"none", nil, nil},
		{"one", []string{"solo"}, []string{"solo"}},
		{"sorted by name", []string{"zeta", "alpha"}, []string{"alpha", "zeta"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := agentHost(t)
			for _, name := range tt.register {
				registerAgent(t, c, name)
			}
			cfgs, err := c.Agents.Agents()
			if err != nil {
				t.Fatalf("Agents() error = %v", err)
			}
			var got []string
			for _, cfg := range cfgs {
				got = append(got, cfg.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Agents() names = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAgentsRuns seeds a running claude-code run on task "t-one" and a
// completed gemini run on task "t-two", then checks every filter.
// Results are newest first.
func TestAgentsRuns(t *testing.T) {
	tests := []struct {
		name string
		opts sdk.RunListOpts
		want []string // task keys in order
	}{
		{"all", sdk.RunListOpts{}, []string{"t-two", "t-one"}},
		{"status running", sdk.RunListOpts{Status: "running"}, []string{"t-one"}},
		{"status completed", sdk.RunListOpts{Status: "completed"}, []string{"t-two"}},
		{"by task", sdk.RunListOpts{TaskKey: "t-one"}, []string{"t-one"}},
		{"by agent", sdk.RunListOpts{Agent: "gemini"}, []string{"t-two"}},
		{"combined", sdk.RunListOpts{Agent: "gemini", Status: "running"}, nil},
		{"no match", sdk.RunListOpts{Status: "stopped"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentHost(t)
			seedRun(t, h, agents.RecordOpts{TaskKey: "t-one", Agent: "claude-code", PID: 4242, Branch: "task/one"})
			seedRun(t, h, agents.RecordOpts{TaskKey: "t-two", Agent: "gemini"})
			if err := c.Agents.Complete("t-two", 0); err != nil {
				t.Fatalf("Complete(t-two) error = %v", err)
			}

			runs, err := c.Agents.Runs(tt.opts)
			if err != nil {
				t.Fatalf("Runs(%+v) error = %v", tt.opts, err)
			}
			var got []string
			for _, r := range runs {
				got = append(got, r.TaskKey)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Runs(%+v) tasks = %v, want %v", tt.opts, got, tt.want)
			}
			for _, r := range runs {
				if r.TaskKey == "t-one" && (r.Status != sdk.AgentRunning || r.PID != 4242 || r.Branch != "task/one" || r.ExitCode != -1) {
					t.Errorf("Runs(%+v) t-one = %+v, want running, pid 4242, branch task/one, exit -1", tt.opts, r)
				}
				if r.TaskKey == "t-two" && (r.Status != sdk.AgentCompleted || r.PID != 0 || r.ExitCode != 0 || r.StoppedAt == 0) {
					t.Errorf("Runs(%+v) t-two = %+v, want completed, pid 0, exit 0, stopped_at set", tt.opts, r)
				}
			}
		})
	}
}

// TestAgentsPrompt covers the WritePrompt/Prompt round trip and the
// fallback to the default template seeded by registration.
func TestAgentsPrompt(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, c sdk.Context)
		agent    string
		role     string
		want     string // exact content; empty means any non-empty content
		wantPath string // path suffix
		wantErr  error
	}{
		{
			name: "written prompt is read back",
			setup: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.WritePrompt("mine", "developer", "# Do the work", "alice"); err != nil {
					t.Fatalf("WritePrompt: %v", err)
				}
			},
			agent:    "mine",
			role:     "developer",
			want:     "# Do the work",
			wantPath: filepath.Join("agents", "mine", "developer.md"),
		},
		{
			name: "write overwrites",
			setup: func(t *testing.T, c sdk.Context) {
				for _, content := range []string{"first", "second"} {
					if err := c.Agents.WritePrompt("mine", "auditor", content, "alice"); err != nil {
						t.Fatalf("WritePrompt(%q): %v", content, err)
					}
				}
			},
			agent:    "mine",
			role:     "auditor",
			want:     "second",
			wantPath: filepath.Join("agents", "mine", "auditor.md"),
		},
		{
			name:     "falls back to the default template",
			setup:    func(t *testing.T, c sdk.Context) { registerAgent(t, c, "seeded") },
			agent:    "unregistered",
			role:     "developer",
			wantPath: filepath.Join("agents", "default", "developer.md"),
		},
		{
			name:    "no template anywhere",
			setup:   func(*testing.T, sdk.Context) {},
			agent:   "nobody",
			role:    "developer",
			wantErr: sdk.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := agentHost(t)
			tt.setup(t, c)
			content, path, err := c.Agents.Prompt(tt.agent, tt.role)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Prompt(%q, %q) error = %v, want %v", tt.agent, tt.role, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if tt.want != "" && content != tt.want {
				t.Errorf("Prompt(%q, %q) = %q, want %q", tt.agent, tt.role, content, tt.want)
			}
			if tt.want == "" && content == "" {
				t.Errorf("Prompt(%q, %q) is empty, want the seeded template", tt.agent, tt.role)
			}
			if !strings.HasSuffix(path, tt.wantPath) {
				t.Errorf("Prompt(%q, %q) path = %q, want suffix %q", tt.agent, tt.role, path, tt.wantPath)
			}
		})
	}
}

// TestAgentsComplete covers recording the outcome of a running run,
// stats extracted from the agent's log, and the not-found and
// not-running errors.
func TestAgentsComplete(t *testing.T) {
	tests := []struct {
		name       string
		seed       bool
		agent      string
		log        string                            // written to <worktree>/.llmd-agent.log when set
		inPlace    bool                              // the run has no worktree
		before     func(t *testing.T, c sdk.Context) // runs after seeding
		exitCode   int
		wantErr    error
		wantStatus string
		wantStats  bool
	}{
		{name: "exit zero completes", seed: true, agent: "claude-code", exitCode: 0, wantStatus: sdk.AgentCompleted},
		{name: "non-zero exit fails", seed: true, agent: "claude-code", exitCode: 3, wantStatus: sdk.AgentFailed},
		{name: "stats from the log", seed: true, agent: "claude-code", log: claudeLog, exitCode: 0, wantStatus: sdk.AgentCompleted, wantStats: true},
		{name: "platform without stats", seed: true, agent: "my-agent", log: claudeLog, exitCode: 0, wantStatus: sdk.AgentCompleted},
		// A run in the main checkout records no worktree; its log is in
		// .llmd/runs/<task>/.
		{name: "stats from an in-place run", seed: true, inPlace: true, agent: "claude-code", log: claudeLog, exitCode: 0, wantStatus: sdk.AgentCompleted, wantStats: true},
		{name: "no run", wantErr: sdk.ErrNotFound},
		{
			name: "already completed", seed: true, agent: "claude-code",
			before: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.Complete("task", 0); err != nil {
					t.Fatalf("Complete: %v", err)
				}
			},
			wantErr: sdk.ErrAgentNotRunning,
		},
		{
			name: "stopped", seed: true, agent: "claude-code",
			before: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.Stop("task", "alice"); err != nil {
					t.Fatalf("Stop: %v", err)
				}
			},
			wantErr: sdk.ErrAgentNotRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentHost(t)
			worktree := t.TempDir()
			logDir := worktree
			if tt.inPlace {
				worktree = ""
				dir, err := newAgentAPI(h.current(), h.git, true, t.Context()).runDir("task")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				logDir = dir
			}
			if tt.seed {
				seedRun(t, h, agents.RecordOpts{TaskKey: "task", Agent: tt.agent, Worktree: worktree})
			}
			if tt.log != "" {
				if err := os.WriteFile(filepath.Join(logDir, ".llmd-agent.log"), []byte(tt.log), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.before != nil {
				tt.before(t, c)
			}

			err := c.Agents.Complete("task", tt.exitCode)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Complete(task, %d) error = %v, want %v", tt.exitCode, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			r, err := c.Agents.Run("task")
			if err != nil {
				t.Fatalf("Run(task) error = %v", err)
			}
			if r.Status != tt.wantStatus || r.ExitCode != tt.exitCode || r.PID != 0 {
				t.Errorf("Run(task) after Complete = status %q exit %d pid %d, want %q %d 0", r.Status, r.ExitCode, r.PID, tt.wantStatus, tt.exitCode)
			}
			gotStats := r.MonetaryCost != nil || r.Model != "" || r.SessionID != "" || r.InputTokens != nil
			if gotStats != tt.wantStats {
				t.Errorf("Run(task) stats present = %v, want %v (%+v)", gotStats, tt.wantStats, r)
			}
			if tt.wantStats && (*r.MonetaryCost != 0.42 || r.Model != "claude-test" || r.SessionID != "sess-1" || *r.InputTokens != 10 || *r.OutputTokens != 20) {
				t.Errorf("Run(task) stats = cost %v model %q session %q in %v out %v, want 0.42 claude-test sess-1 10 20",
					*r.MonetaryCost, r.Model, r.SessionID, *r.InputTokens, *r.OutputTokens)
			}
		})
	}
}

// TestAgentsStop covers stopping a running run with and without a
// live process and a worktree, the not-found error, and the error for
// a run that is no longer running.
func TestAgentsStop(t *testing.T) {
	tests := []struct {
		name     string
		seed     bool
		process  bool   // start a sleeping process and record its PID
		exited   bool   // let the process exit before Stop
		worktree string // recorded worktree path
		before   func(t *testing.T, c sdk.Context)
		wantErr  error
		wantMsg  string
	}{
		{name: "no process, no worktree", seed: true},
		{name: "terminates the process", seed: true, process: true},
		{name: "process already gone", seed: true, process: true, exited: true},
		{name: "worktree removal failure is logged", seed: true, worktree: filepath.Join(os.TempDir(), "not-a-worktree")},
		{name: "no run", wantErr: sdk.ErrNotFound},
		{
			name: "already stopped", seed: true,
			before: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.Stop("task", "alice"); err != nil {
					t.Fatalf("Stop: %v", err)
				}
			},
			wantMsg: "not running",
		},
		{
			name: "already completed", seed: true,
			before: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.Complete("task", 0); err != nil {
					t.Fatalf("Complete: %v", err)
				}
			},
			wantMsg: "not running",
		},
		{
			name: "already completed maps to the SDK sentinel", seed: true,
			before: func(t *testing.T, c sdk.Context) {
				if err := c.Agents.Complete("task", 0); err != nil {
					t.Fatalf("Complete: %v", err)
				}
			},
			wantErr: sdk.ErrAgentNotRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentHost(t)
			var cmd *exec.Cmd
			pid := 0
			if tt.process {
				if runtime.GOOS == "windows" {
					t.Skip("the test child is the Unix sleep command")
				}
				arg := "60"
				if tt.exited {
					arg = "0"
				}
				cmd = exec.Command("sleep", arg)
				if err := cmd.Start(); err != nil {
					t.Skipf("cannot start sleep: %v", err)
				}
				pid = cmd.Process.Pid
				if tt.exited {
					// Reaped, so the signals Stop sends have no target.
					if err := cmd.Wait(); err != nil {
						t.Fatalf("sleep 0: %v", err)
					}
					cmd = nil
				} else {
					t.Cleanup(func() { _ = cmd.Process.Kill() })
				}
			}
			if tt.seed {
				seedRun(t, h, agents.RecordOpts{TaskKey: "task", Agent: "claude-code", PID: pid, Worktree: tt.worktree})
			}
			if tt.before != nil {
				tt.before(t, c)
			}

			err := c.Agents.Stop("task", "alice")
			if tt.wantErr != nil || tt.wantMsg != "" {
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("Stop(task) error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
					t.Errorf("Stop(task) error = %v, want to contain %q", err, tt.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Stop(task) error = %v", err)
			}
			r, err := c.Agents.Run("task")
			if err != nil {
				t.Fatalf("Run(task) error = %v", err)
			}
			if r.Status != sdk.AgentStopped || r.PID != 0 || r.StoppedAt == 0 {
				t.Errorf("Run(task) after Stop = %+v, want stopped with pid 0 and stopped_at set", r)
			}
			if cmd != nil {
				done := make(chan error, 1)
				go func() { done <- cmd.Wait() }()
				select {
				case werr := <-done:
					if werr == nil {
						t.Errorf("sleep exited cleanly, want it terminated by Stop")
					}
				case <-time.After(5 * time.Second):
					t.Errorf("sleep still running 5s after Stop")
				}
			}
		})
	}
}

// TestBuildResumePrompt covers the prompt a resumed agent receives:
// generic when nothing was said since its last run, otherwise the
// audit feedback recorded since then, skipping empty entries.
func TestBuildResumePrompt(t *testing.T) {
	tests := []struct {
		name        string
		audits      []sdk.AuditOpts // Target is filled with the task key
		sinceOffset time.Duration   // added to the time before the audits
		wantGeneric bool
		wantParts   []string
		wantAbsent  []string
	}{
		{name: "no audits", wantGeneric: true},
		{
			name: "feedback since the previous run",
			audits: []sdk.AuditOpts{
				{Author: "auditor", Content: "Tests are missing.", Status: "needs-work"},
				{Author: "lead", Content: "Also rename the flag."},
			},
			wantParts:  []string{"was reviewed", "--- auditor (needs-work):\nTests are missing.", "--- lead (pending):\nAlso rename the flag."},
			wantAbsent: []string{"Resume work on task"},
		},
		{
			name: "empty entries are skipped",
			audits: []sdk.AuditOpts{
				{Author: "auditor", Content: ""},
				{Author: "lead", Content: "Fix it."},
			},
			wantParts:  []string{"--- lead (pending):\nFix it."},
			wantAbsent: []string{"--- auditor"},
		},
		{
			name:        "audits before the previous run are ignored",
			audits:      []sdk.AuditOpts{{Author: "auditor", Content: "old news"}},
			sinceOffset: time.Hour,
			wantGeneric: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentHost(t)
			task := addTask(t, c, "Resumable", spec, sdk.TaskAddOpts{Author: "alice"})
			since := time.Now().Add(-time.Second).Add(tt.sinceOffset).UnixMilli()
			for _, opts := range tt.audits {
				opts.Target = task.Key
				if _, err := c.Audits.Add(opts); err != nil {
					t.Fatalf("Audits.Add(%+v) error = %v", opts, err)
				}
			}

			a := newAgentAPI(h.current(), h.git, true, t.Context())
			got := a.buildResumePrompt(task.Key, since)
			generic := "Resume work on task " + task.Key + ". Review any recent feedback and address outstanding issues."
			if tt.wantGeneric {
				if got != generic {
					t.Errorf("buildResumePrompt() = %q, want %q", got, generic)
				}
				return
			}
			if !strings.HasPrefix(got, "Your previous work on task "+task.Key+" was reviewed.") {
				t.Errorf("buildResumePrompt() = %q, want the review header", got)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("buildResumePrompt() = %q, want to contain %q", got, part)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("buildResumePrompt() = %q, must not contain %q", got, absent)
				}
			}
		})
	}
}

func TestHasFlag(t *testing.T) {
	tests := []struct {
		flags string
		flag  string
		want  bool
	}{
		{"", "resume", false},
		{"resume", "resume", true},
		{"blocked,resume", "resume", true},
		{"blocked, resume ", "resume", true},
		{"resumed", "resume", false},
		{"blocked", "resume", false},
		{"resume", "", false},
		{"a,,b", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.flags+"/"+tt.flag, func(t *testing.T) {
			if got := hasFlag(tt.flags, tt.flag); got != tt.want {
				t.Errorf("hasFlag(%q, %q) = %v, want %v", tt.flags, tt.flag, got, tt.want)
			}
		})
	}
}

// TestRunsInPlace checks where a spawned agent works: auditors always
// in the main checkout, other roles by the spawn's override when one is
// given and by agent.worktree otherwise.
func TestRunsInPlace(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		worktree bool
		override *bool
		want     bool
	}{
		{"worktree from config", "developer", true, nil, false},
		{"main checkout from config", "developer", false, nil, true},
		{"override turns worktree on", "developer", false, new(true), false},
		{"override turns worktree off", "developer", true, new(false), true},
		{"auditor ignores config", "auditor", true, nil, true},
		{"auditor ignores override", "auditor", true, new(true), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runsInPlace(tt.role, tt.worktree, tt.override); got != tt.want {
				t.Errorf("runsInPlace(%q, %v, %v) = %v, want %v", tt.role, tt.worktree, tt.override, got, tt.want)
			}
		})
	}
}

// TestWriteSettings covers writing an agent's settings file, creating
// its directory, and the failures that leave the agent without one.
func TestWriteSettings(t *testing.T) {
	tests := []struct {
		name string
		path func(t *testing.T) string
		want bool
	}{
		{"creates the directory", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), ".claude", "settings.json")
		}, true},
		{"unwritable directory", func(t *testing.T) string {
			// A regular file where the directory should be.
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(file, "settings.json")
		}, false},
		{"unwritable file", func(t *testing.T) string {
			// A directory where the settings file should be.
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return path
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path(t)
			if got := writeSettings(path, `{"hooks":{}}`); got != tt.want {
				t.Fatalf("writeSettings(%s) = %v, want %v", path, got, tt.want)
			}
			if !tt.want {
				return
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != `{"hooks":{}}` {
				t.Errorf("ReadFile(%s) = %q, %v, want the settings content", path, data, err)
			}
		})
	}
}

// TestRunToSDK checks the field-for-field mapping of an internal run.
func TestRunToSDK(t *testing.T) {
	cost, in, out := 1.5, 100, 200
	r := &agents.Run{
		Key: "run1", TaskKey: "task1", Agent: "claude-code", Branch: "task/x", Worktree: "/wt",
		Status: "completed", PID: 0, ExitCode: 0, MonetaryCost: &cost, InputTokens: &in, OutputTokens: &out,
		Model: "m", SessionID: "s", Author: "alice", StartedAt: 10, StoppedAt: 20,
	}
	want := &sdk.AgentRun{
		Key: "run1", TaskKey: "task1", Agent: "claude-code", Branch: "task/x", Worktree: "/wt",
		Status: "completed", PID: 0, ExitCode: 0, MonetaryCost: &cost, InputTokens: &in, OutputTokens: &out,
		Model: "m", SessionID: "s", Author: "alice", StartedAt: 10, StoppedAt: 20,
	}
	if got := runToSDK(r); !reflect.DeepEqual(got, want) {
		t.Errorf("runToSDK(%+v) = %+v, want %+v", r, got, want)
	}
}

// TestAgentErr covers the translation of every agents sentinel.
func TestAgentErr(t *testing.T) {
	other := errors.New("disk on fire")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"agent not found", agents.ErrNotFound, sdk.ErrNotFound},
		{"run not found", agents.ErrRunNotFound, sdk.ErrNotFound},
		{"running", agents.ErrRunning, sdk.ErrAgentRunning},
		{"not running", agents.ErrNotRunning, sdk.ErrAgentNotRunning},
		{"passthrough", other, other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentErr(tt.in)
			if !errors.Is(got, tt.want) {
				t.Errorf("agentErr(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.in != nil && !errors.Is(got, tt.in) {
				t.Errorf("agentErr(%v) = %v, want to wrap the original", tt.in, got)
			}
		})
	}
}
