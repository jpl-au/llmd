package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/agents"
	"github.com/jpl-au/llmd/host"
	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/sqlite/sqlitetest"
)

// agentSetup is newCLI that also returns the host, for tests that need
// the store directory (agent files live under .llmd/agents).
func agentSetup(t *testing.T, author string) (*host.Host, sdk.Context) {
	t.Helper()
	h := host.TestSetup(t, host.TestIsolated)
	return h, h.Context(t.Context(), author)
}

// agentDir returns the directory that holds agent configuration files.
func agentDir(h *host.Host) string {
	return filepath.Join(h.Dir(), ".llmd", "agents")
}

// agentSeedRun records a running agent for taskKey without spawning a
// process, through a second handle on the same store. PID 0 and no
// worktree mean stop has nothing to signal or clean up.
func agentSeedRun(t *testing.T, h *host.Host, taskKey, agent string, pid int) {
	t.Helper()
	store, err := engine.Open(filepath.Join(h.Dir(), ".llmd", "llmd.db"), engine.WithDatabase(sqlitetest.Settings()))
	if err != nil {
		t.Fatalf("engine.Open: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("closing seed store: %v", err)
		}
	}()
	if _, err := store.Agents.Record(t.Context(), agents.RecordOpts{
		TaskKey: taskKey, Agent: agent, PID: pid, Author: "seed",
	}); err != nil {
		t.Fatalf("Record(%s): %v", taskKey, err)
	}
}

// agentRunsOf returns the []*sdk.AgentRun carried by an agent runs response.
func agentRunsOf(t *testing.T, resp sdk.Response) []*sdk.AgentRun {
	t.Helper()
	rr, ok := data(resp).([]*sdk.AgentRun)
	if !ok {
		t.Fatalf("agent runs Data = %T, want []*sdk.AgentRun", data(resp))
	}
	return rr
}

// TestAgentCmd covers the dispatcher: subcommand required, unknown
// subcommands rejected, and the author rule (stop, spawn and complete
// need one; add and rm default to "llmd").
func TestAgentCmd(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		args    []string
		wantErr error
		wantMsg string
	}{
		{name: "no subcommand", author: "alice", wantErr: sdk.ErrMissingArg},
		{name: "unknown subcommand", author: "alice", args: []string{"bogus"}, wantMsg: "unknown subcommand"},
		{name: "stop needs author", args: []string{"stop", "abc"}, wantMsg: "an author is required."},
		{name: "complete needs author", args: []string{"complete", "abc"}, wantMsg: "an author is required."},
		{name: "spawn needs author", args: []string{"spawn", "abc", "claude-code"}, wantMsg: "an author is required."},
		{name: "spawn needs task and agent", author: "alice", args: []string{"spawn", "abc"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, tt.author)
			err := runErr(t, c, "agent", nil, tt.args...)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
				t.Errorf("agent %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
			}
		})
	}
}

// TestAgentAdd covers built-in profiles, custom commands, the files
// written under .llmd/agents and the argument errors. No author is
// set: add is configuration and defaults to "llmd".
func TestAgentAdd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantText    []string
		wantCommand string   // from agent config afterwards
		wantArgs    string   // substring of the joined args
		wantFiles   []string // relative to the agents directory
		wantNoFiles []string
		wantErr     error
		wantMsg     string
	}{
		{
			name: "built-in claude-code",
			args: []string{"add", "claude-code"},
			wantText: []string{
				"Registered claude-code", "Command:  claude",
				filepath.Join(".llmd", "agents", "claude-code", "config.json"),
				filepath.Join(".llmd", "agents", "claude-code", "developer.md"),
				filepath.Join(".llmd", "agents", "claude-code", "auditor.md"),
				sdk.Program() + " task start <task-id> --agent claude-code",
			},
			wantCommand: "claude",
			wantArgs:    "--output-format json",
			wantFiles: []string{
				"claude-code/config.json", "claude-code/settings.json",
				"claude-code/developer.md", "claude-code/auditor.md", "claude-code/tester.md",
				"default/developer.md", "default/auditor.md", "default/tester.md",
			},
		},
		{
			name:        "built-in gemini",
			args:        []string{"add", "gemini"},
			wantText:    []string{"Registered gemini", "Command:  gemini"},
			wantCommand: "gemini",
			wantArgs:    "--yolo",
			wantFiles:   []string{"gemini/config.json", "gemini/developer.md"},
			wantNoFiles: []string{"gemini/settings.json"},
		},
		{
			name:        "custom agent",
			args:        []string{"add", "test-agent", "--command", "/usr/bin/echo"},
			wantText:    []string{"Registered test-agent", "Command:  /usr/bin/echo"},
			wantCommand: "/usr/bin/echo",
			wantArgs:    "-p {{.Prompt}}",
			wantFiles:   []string{"test-agent/config.json", "test-agent/developer.md", "default/developer.md"},
			wantNoFiles: []string{"test-agent/settings.json"},
		},
		{
			name:        "--command overrides a built-in profile",
			args:        []string{"add", "claude-code", "--command", "/opt/claude"},
			wantText:    []string{"Registered claude-code", "Command:  /opt/claude"},
			wantCommand: "/opt/claude",
			wantArgs:    "-p {{.Prompt}}",
		},
		{
			name:    "unknown agent without --command",
			args:    []string{"add", "mystery"},
			wantMsg: "not a built-in agent",
		},
		{
			name:    "no name lists the built-ins",
			args:    []string{"add"},
			wantErr: sdk.ErrMissingArg,
			wantMsg: "aider, claude-code, gemini",
		},
		{
			name:    "unknown flag",
			args:    []string{"add", "claude-code", "--status", "x"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "command without value",
			args:    []string{"add", "x", "--command"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "")
			if tt.wantErr != nil || tt.wantMsg != "" {
				err := runErr(t, c, "agent", nil, tt.args...)
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
					t.Errorf("agent %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
				}
				if _, err := os.Stat(agentDir(h)); !os.IsNotExist(err) {
					t.Errorf("agent %v created %s, want nothing on disk", tt.args, agentDir(h))
				}
				return
			}

			resp := run(t, c, "agent", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("agent %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("agent %v response type = %T, want sdk.Text", tt.args, resp)
			}
			for _, f := range tt.wantFiles {
				if _, err := os.Stat(filepath.Join(agentDir(h), f)); err != nil {
					t.Errorf("agent %v did not write %s: %v", tt.args, f, err)
				}
			}
			for _, f := range tt.wantNoFiles {
				if _, err := os.Stat(filepath.Join(agentDir(h), f)); err == nil {
					t.Errorf("agent %v wrote %s, want absent", tt.args, f)
				}
			}

			cfg, ok := data(run(t, c, "agent", nil, "config", tt.args[1])).(*sdk.AgentConfig)
			if !ok || cfg == nil {
				t.Fatalf("agent config Data = %T, want *sdk.AgentConfig", cfg)
			}
			if cfg.Name != tt.args[1] || cfg.Command != tt.wantCommand {
				t.Errorf("agent config = %+v, want name %q command %q", cfg, tt.args[1], tt.wantCommand)
			}
			if joined := strings.Join(cfg.Args, " "); !strings.Contains(joined, tt.wantArgs) {
				t.Errorf("agent config Args = %q, want to contain %q", joined, tt.wantArgs)
			}
			if got := text(run(t, c, "agent", nil, "ls")); !strings.Contains(got, tt.args[1]) {
				t.Errorf("agent ls = %q, want to contain %q", got, tt.args[1])
			}
		})
	}

	t.Run("re-adding replaces the configuration", func(t *testing.T) {
		_, c := agentSetup(t, "")
		run(t, c, "agent", nil, "add", "claude-code")
		run(t, c, "agent", nil, "add", "claude-code", "--command", "/opt/claude")
		cfg := data(run(t, c, "agent", nil, "config", "claude-code")).(*sdk.AgentConfig)
		if cfg.Command != "/opt/claude" {
			t.Errorf("agent config Command = %q, want the replacement /opt/claude", cfg.Command)
		}
		if n := len(data(run(t, c, "agent", nil, "ls")).([]sdk.AgentConfig)); n != 1 {
			t.Errorf("agent ls lists %d agents, want 1", n)
		}
	})
}

// TestAgentLs covers the empty message, the table of registered agents
// and the list alias.
func TestAgentLs(t *testing.T) {
	tests := []struct {
		name       string
		agents     [][]string // add arguments per agent
		args       []string
		wantText   []string
		wantAbsent []string
		wantLen    int
	}{
		{
			name:     "none registered",
			args:     []string{"ls"},
			wantText: []string{"No agents registered", sdk.Program() + " agent add claude-code"},
		},
		{
			name:     "one agent",
			agents:   [][]string{{"claude-code"}},
			args:     []string{"ls"},
			wantText: []string{"NAME", "COMMAND", "claude-code", "claude"},
			wantLen:  1,
		},
		{
			name:     "two agents",
			agents:   [][]string{{"claude-code"}, {"test-agent", "--command", "/usr/bin/echo"}},
			args:     []string{"list"},
			wantText: []string{"claude-code", "test-agent", "/usr/bin/echo"},
			wantLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := agentSetup(t, "")
			for _, a := range tt.agents {
				run(t, c, "agent", nil, append([]string{"add"}, a...)...)
			}
			resp := run(t, c, "agent", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("agent %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			if tt.wantLen == 0 {
				if data(resp) != nil {
					t.Errorf("agent %v Data = %v, want nil for the empty message", tt.args, data(resp))
				}
				return
			}
			cfgs, ok := data(resp).([]sdk.AgentConfig)
			if !ok || len(cfgs) != tt.wantLen {
				t.Errorf("agent %v Data = %T with %d entries, want []sdk.AgentConfig of %d", tt.args, data(resp), len(cfgs), tt.wantLen)
			}
			// The default prompt directory is not an agent.
			if strings.Contains(got, "default") {
				t.Errorf("agent %v = %q, must not list the default prompt directory", tt.args, got)
			}
		})
	}
}

// TestAgentConfig covers the field table, the structured payload and
// the error cases.
func TestAgentConfig(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantText   []string
		wantAbsent []string
		wantErr    error
	}{
		{
			name:       "built-in profile",
			args:       []string{"config", "claude-code"},
			wantText:   []string{"FIELD", "VALUE", "name", "claude-code", "command", "claude", "args", "--output-format json"},
			wantAbsent: []string{"role", "budget"},
		},
		{
			name:     "custom agent",
			args:     []string{"config", "test-agent"},
			wantText: []string{"test-agent", "/usr/bin/echo", "-p {{.Prompt}}"},
		},
		{name: "missing name", args: []string{"config"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown agent", args: []string{"config", "nobody"}, wantErr: sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := agentSetup(t, "")
			run(t, c, "agent", nil, "add", "claude-code")
			run(t, c, "agent", nil, "add", "test-agent", "--command", "/usr/bin/echo")

			if tt.wantErr != nil {
				if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("agent %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("agent %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			cfg, ok := data(resp).(*sdk.AgentConfig)
			if !ok || cfg == nil || cfg.Name != tt.args[1] || cfg.Command == "" {
				t.Errorf("agent %v Data = %+v, want *sdk.AgentConfig for %s with a command", tt.args, data(resp), tt.args[1])
			}
		})
	}
}

// TestAgentConfigOptionalRows covers the role and budget rows, which
// only appear when the stored configuration sets them.
func TestAgentConfigOptionalRows(t *testing.T) {
	_, c := agentSetup(t, "")
	if err := c.Agents.Register(sdk.AgentConfig{Name: "reviewer", Command: "/usr/bin/review", Role: "auditor", MaxBudget: 2.5}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got := text(run(t, c, "agent", nil, "config", "reviewer"))
	for _, want := range []string{"role", "auditor", "budget", "$2.50"} {
		if !strings.Contains(got, want) {
			t.Errorf("agent config reviewer = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "args") {
		t.Errorf("agent config reviewer = %q, must not show an args row when there are none", got)
	}
}

// TestAgentPrompt covers the agent-specific template, the fallback to
// the shared default directory, the structured payload and the errors.
func TestAgentPrompt(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string // substring of the reported path
		wantErr  error
	}{
		{name: "developer template", args: []string{"prompt", "claude-code", "developer"}, wantPath: filepath.Join("claude-code", "developer.md")},
		{name: "auditor template", args: []string{"prompt", "claude-code", "auditor"}, wantPath: filepath.Join("claude-code", "auditor.md")},
		{name: "tester template", args: []string{"prompt", "claude-code", "tester"}, wantPath: filepath.Join("claude-code", "tester.md")},
		{name: "falls back to the default directory", args: []string{"prompt", "unregistered", "developer"}, wantPath: filepath.Join("default", "developer.md")},
		{name: "unknown role", args: []string{"prompt", "claude-code", "poet"}, wantErr: sdk.ErrNotFound},
		{name: "missing role", args: []string{"prompt", "claude-code"}, wantErr: sdk.ErrMissingArg},
		{name: "missing name and role", args: []string{"prompt"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "")
			run(t, c, "agent", nil, "add", "claude-code")

			if tt.wantErr != nil {
				if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			got := text(resp)
			header := "# " + tt.args[1] + "/" + tt.args[2] + " (from "
			if !strings.HasPrefix(got, header) {
				t.Errorf("agent %v = %q, want prefix %q", tt.args, got, header)
			}
			if !strings.Contains(got, tt.wantPath) {
				t.Errorf("agent %v = %q, want to contain the path %q", tt.args, got, tt.wantPath)
			}
			d, ok := data(resp).(map[string]string)
			if !ok {
				t.Fatalf("agent %v Data = %T, want map[string]string", tt.args, data(resp))
			}
			if d["name"] != tt.args[1] || d["role"] != tt.args[2] {
				t.Errorf("agent %v Data name/role = %q/%q, want %q/%q", tt.args, d["name"], d["role"], tt.args[1], tt.args[2])
			}
			if !strings.HasPrefix(d["path"], agentDir(h)) || !strings.HasSuffix(d["path"], tt.wantPath) {
				t.Errorf("agent %v Data path = %q, want under %s ending in %s", tt.args, d["path"], agentDir(h), tt.wantPath)
			}
			if d["content"] == "" || !strings.HasSuffix(got, d["content"]) {
				t.Errorf("agent %v Data content = %q, want the template that the text ends with", tt.args, d["content"])
			}
			// The seeded templates come from the embedded defaults.
			if !strings.Contains(d["content"], "{{.Key}}") {
				t.Errorf("agent %v content = %q, want the default template with {{.Key}}", tt.args, d["content"])
			}
		})
	}

	t.Run("nothing registered", func(t *testing.T) {
		_, c := agentSetup(t, "")
		if err := runErr(t, c, "agent", nil, "prompt", "claude-code", "developer"); !errors.Is(err, sdk.ErrNotFound) {
			t.Errorf("agent prompt error = %v, want %v", err, sdk.ErrNotFound)
		}
	})
}

// TestAgentRm covers removal (files, ls and config all agree), the
// remove alias and the error cases.
func TestAgentRm(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "removes the agent", args: []string{"rm", "test-agent"}},
		{name: "remove alias", args: []string{"remove", "test-agent"}},
		{name: "unknown agent", args: []string{"rm", "nobody"}, wantErr: sdk.ErrNotFound},
		{name: "missing name", args: []string{"rm"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "")
			run(t, c, "agent", nil, "add", "claude-code")
			run(t, c, "agent", nil, "add", "test-agent", "--command", "/usr/bin/echo")

			if tt.wantErr != nil {
				if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			if got, want := text(resp), `Removed agent "test-agent"`; got != want {
				t.Errorf("agent %v = %q, want %q", tt.args, got, want)
			}
			if got := text(run(t, c, "agent", nil, "ls")); strings.Contains(got, "test-agent") || !strings.Contains(got, "claude-code") {
				t.Errorf("agent ls after rm = %q, want claude-code only", got)
			}
			if err := runErr(t, c, "agent", nil, "config", "test-agent"); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("agent config after rm error = %v, want %v", err, sdk.ErrNotFound)
			}
			if _, err := os.Stat(filepath.Join(agentDir(h), "test-agent")); !os.IsNotExist(err) {
				t.Errorf("agent directory still exists after rm: %v", err)
			}
			// Removing twice is not found.
			if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("agent %v twice error = %v, want %v", tt.args, err, sdk.ErrNotFound)
			}
		})
	}
}

// TestAgentRuns covers the empty message, every filter and the table
// rendering of running and finished runs. Fixture when seeded: task
// "task00001" running under claude-code (pid 4242) and task
// "task00002" completed under gemini.
func TestAgentRuns(t *testing.T) {
	tests := []struct {
		name       string
		seed       bool
		args       []string
		wantText   []string
		wantAbsent []string
		wantLen    int
		wantErr    error
	}{
		{name: "none", args: []string{"runs"}, wantText: []string{"No agent runs"}},
		{name: "status filter accepted when empty", args: []string{"runs", "--status", "running"}, wantText: []string{"No agent runs"}},
		{name: "task filter accepted when empty", args: []string{"runs", "--task", "task00001"}, wantText: []string{"No agent runs"}},
		{name: "agent filter accepted when empty", args: []string{"runs", "--agent", "claude-code"}, wantText: []string{"No agent runs"}},
		{
			name:     "all runs",
			seed:     true,
			args:     []string{"runs"},
			wantText: []string{"RUN", "AGENT", "TASK", "STATUS", "BRANCH", "claude-code", "task00001", "running (pid 4242)", "gemini", "task00002", "completed"},
			wantLen:  2,
		},
		{
			name:       "status running",
			seed:       true,
			args:       []string{"runs", "--status", "running"},
			wantText:   []string{"task00001", "running (pid 4242)"},
			wantAbsent: []string{"task00002", "completed"},
			wantLen:    1,
		},
		{
			name:       "status completed",
			seed:       true,
			args:       []string{"runs", "--status", "completed"},
			wantText:   []string{"task00002", "completed"},
			wantAbsent: []string{"task00001"},
			wantLen:    1,
		},
		{
			name:       "by agent",
			seed:       true,
			args:       []string{"runs", "--agent", "gemini"},
			wantText:   []string{"task00002"},
			wantAbsent: []string{"task00001"},
			wantLen:    1,
		},
		{
			name:       "by task",
			seed:       true,
			args:       []string{"runs", "--task", "task00001"},
			wantText:   []string{"claude-code"},
			wantAbsent: []string{"gemini"},
			wantLen:    1,
		},
		{name: "no match", seed: true, args: []string{"runs", "--status", "stopped"}, wantText: []string{"No agent runs"}},
		{name: "unknown flag", args: []string{"runs", "--all"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "alice")
			if tt.seed {
				agentSeedRun(t, h, "task00001", "claude-code", 4242)
				agentSeedRun(t, h, "task00002", "gemini", 0)
				run(t, c, "agent", nil, "complete", "task00002", "--exit-code", "0")
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("agent %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("agent %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if tt.wantLen == 0 {
				if data(resp) != nil {
					t.Errorf("agent %v Data = %v, want nil for the empty message", tt.args, data(resp))
				}
				return
			}
			runs := agentRunsOf(t, resp)
			if len(runs) != tt.wantLen {
				t.Errorf("agent %v Data has %d runs, want %d", tt.args, len(runs), tt.wantLen)
			}
			for _, r := range runs {
				if r.Key == "" || r.Status == "" || !strings.Contains(got, r.Key) {
					t.Errorf("agent %v run %+v: want a key and status shown in the table", tt.args, r)
				}
			}
		})
	}
}

// TestAgentComplete covers recording completion and failure for a
// running agent, the argument errors, and the not-found / not-running
// mappings.
func TestAgentComplete(t *testing.T) {
	tests := []struct {
		name       string
		seed       bool
		done       bool // complete once before the call
		args       []string
		wantText   string
		wantStatus string
		wantErr    error
	}{
		{
			name: "exit zero completes", seed: true,
			args:     []string{"complete", "task00001"},
			wantText: "Completed agent run for task task00001 (exit 0)", wantStatus: "completed",
		},
		{
			name: "explicit exit code zero", seed: true,
			args:     []string{"complete", "task00001", "--exit-code", "0"},
			wantText: "Completed agent run for task task00001 (exit 0)", wantStatus: "completed",
		},
		{
			name: "non-zero exit fails", seed: true,
			args:     []string{"complete", "task00001", "--exit-code", "3"},
			wantText: "Completed agent run for task task00001 (exit 3)", wantStatus: "failed",
		},
		{name: "no run for the task", args: []string{"complete", "task00001"}, wantErr: sdk.ErrNotFound},
		{name: "already finished", seed: true, done: true, args: []string{"complete", "task00001"}, wantErr: sdk.ErrAgentNotRunning},
		{name: "missing task key", seed: true, args: []string{"complete"}, wantErr: sdk.ErrMissingArg},
		{name: "exit code must be an integer", seed: true, args: []string{"complete", "task00001", "--exit-code", "ok"}, wantErr: sdk.ErrInvalidArg},
		{name: "unknown flag", seed: true, args: []string{"complete", "task00001", "--status", "x"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "alice")
			if tt.seed {
				agentSeedRun(t, h, "task00001", "claude-code", 0)
			}
			if tt.done {
				run(t, c, "agent", nil, "complete", "task00001")
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "agent", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			if got := text(resp); got != tt.wantText {
				t.Errorf("agent %v = %q, want %q", tt.args, got, tt.wantText)
			}
			runs := agentRunsOf(t, run(t, c, "agent", nil, "runs", "--task", "task00001"))
			if len(runs) != 1 || runs[0].Status != tt.wantStatus {
				t.Errorf("agent runs after complete = %+v, want one run with status %q", runs, tt.wantStatus)
			}
			if got := text(run(t, c, "agent", nil, "runs")); !strings.Contains(got, tt.wantStatus) || strings.Contains(got, "running") {
				t.Errorf("agent runs after complete = %q, want %q and no running entry", got, tt.wantStatus)
			}
		})
	}
}

// TestAgentStop covers stopping a running agent (no process, no
// worktree), the not-found mapping and the argument errors.
func TestAgentStop(t *testing.T) {
	tests := []struct {
		name    string
		seed    bool
		stopped bool // stop once before the call
		args    []string
		wantErr error
		wantMsg string
	}{
		{name: "stops a running agent", seed: true, args: []string{"stop", "task00001"}},
		{name: "no run for the task", args: []string{"stop", "task00001"}, wantErr: sdk.ErrNotFound},
		{name: "already stopped", seed: true, stopped: true, args: []string{"stop", "task00001"}, wantMsg: "not running"},
		{name: "missing task key", seed: true, args: []string{"stop"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, c := agentSetup(t, "alice")
			if tt.seed {
				agentSeedRun(t, h, "task00001", "claude-code", 0)
			}
			if tt.stopped {
				run(t, c, "agent", nil, "stop", "task00001")
			}

			if tt.wantErr != nil || tt.wantMsg != "" {
				err := runErr(t, c, "agent", nil, tt.args...)
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("agent %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
					t.Errorf("agent %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
				}
				return
			}
			resp := run(t, c, "agent", nil, tt.args...)
			if got, want := text(resp), "Stopped agent for task task00001"; got != want {
				t.Errorf("agent %v = %q, want %q", tt.args, got, want)
			}
			runs := agentRunsOf(t, run(t, c, "agent", nil, "runs", "--status", "stopped"))
			if len(runs) != 1 || runs[0].TaskKey != "task00001" || runs[0].PID != 0 {
				t.Errorf("agent runs --status stopped = %+v, want the stopped run for task00001", runs)
			}
			// A stopped run cannot be completed.
			if err := runErr(t, c, "agent", nil, "complete", "task00001"); !errors.Is(err, sdk.ErrAgentNotRunning) {
				t.Errorf("agent complete after stop error = %v, want %v", err, sdk.ErrAgentNotRunning)
			}
		})
	}
}
