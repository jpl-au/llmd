package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/engine/agents"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// spyGit is a GitStore that records whether Spawn reached the git
// check. Spawn consults git only after the task, its readiness and the
// agent configuration have all been resolved, so a recorded call means
// every earlier gate in the pipeline let the spawn through. Available
// fails so the spawn stops there and never touches the filesystem.
type spyGit struct {
	sdk.GitStore
	called bool
}

func (g *spyGit) Available() error {
	g.called = true
	return errors.New("spy: git unavailable")
}

// setRule writes a column rule and fails the test on error.
func setRule(t *testing.T, c sdk.Context, column string, rule sdk.ColumnRule) {
	t.Helper()
	if err := c.Rules.Set(column, rule); err != nil {
		t.Fatalf("Rules.Set(%q, %+v) error = %v", column, rule, err)
	}
}

// setGhostRule leaves the review column's rule naming an agent whose
// configuration is gone: rule set refuses an unknown agent, but one
// can still be removed after its rule was set.
func setGhostRule(t *testing.T, c sdk.Context) {
	t.Helper()
	registerAgent(t, c, "ghost")
	setRule(t, c, "review", sdk.ColumnRule{Agent: "ghost", Success: "approval", Failure: "blocked"})
	if err := c.Agents.Remove("ghost"); err != nil {
		t.Fatalf("Agents.Remove(ghost) error = %v", err)
	}
}

// registerAgent registers an agent whose command is the running test
// binary, which exists on every platform, so Spawn passes the command
// lookup and reaches the git check.
func registerAgent(t *testing.T, c sdk.Context, name string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Agents.Register(sdk.AgentConfig{Name: name, Command: exe}); err != nil {
		t.Fatalf("Agents.Register(%q) error = %v", name, err)
	}
}

// seedRun records a running agent for taskKey directly in the store,
// the way a spawn would, without starting a process.
func seedRun(t *testing.T, h *Host, opts agents.RecordOpts) *agents.Run {
	t.Helper()
	if opts.Author == "" {
		opts.Author = "seed"
	}
	r, err := h.current().Agents.Record(t.Context(), opts)
	if err != nil {
		t.Fatalf("Agents.Record(%+v) error = %v", opts, err)
	}
	return r
}

// runCount returns the number of agent runs recorded for a task.
func runCount(t *testing.T, c sdk.Context, key string) int {
	t.Helper()
	runs, err := c.Agents.Runs(sdk.RunListOpts{TaskKey: key})
	if err != nil {
		t.Fatalf("Agents.Runs(%q) error = %v", key, err)
	}
	return len(runs)
}

// TestPipelineHandleEvent drives the pipeline handler with bus events
// and checks which ones reach a spawn attempt. The handler is built
// with a spy git so the attempt is observable and stops before any
// worktree is created. A spawn that succeeds launches a detached
// "llmd agent run" process, so that path is not exercised here: the
// furthest a test can go is the git check, after which Spawn fails and
// the handler logs the failure and returns nil.
//
// The tests use TestWorkdir so the working directory is a temporary
// directory outside any git repository: Spawn resolves worktree paths
// relative to it.
func TestPipelineHandleEvent(t *testing.T) {
	const agent = "sh-agent"
	agentRule := sdk.ColumnRule{Agent: agent, Role: "developer", Success: "approval", Failure: "blocked"}

	tests := []struct {
		name      string
		setup     func(t *testing.T, h *Host, c sdk.Context, key string)
		event     func(key string) pkgevents.Event
		wantSpawn bool // Spawn reached the git check
		wantRuns  int  // runs recorded for the task afterwards
	}{
		{
			name: "ignores non-task events",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.DocumentWritten, Key: key, Path: "docs/a", Metadata: map[string]any{"to": "review"}}
			},
		},
		{
			name: "ignores a move without a destination",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice"}
			},
		},
		{
			name: "unreadable rules",
			setup: func(t *testing.T, h *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
				path := filepath.Join(h.Dir(), ".llmd", "rules", "default.yaml")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("review: [unterminated\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
		},
		{
			name: "manual column",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
		},
		{
			name: "column without any rule",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "done"}}
			},
		},
		{
			name: "agent already running",
			setup: func(t *testing.T, h *Host, c sdk.Context, key string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
				seedRun(t, h, agents.RecordOpts{TaskKey: key, Agent: agent})
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
			wantRuns: 1,
		},
		{
			name: "finished run does not block a new spawn",
			setup: func(t *testing.T, h *Host, c sdk.Context, key string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
				seedRun(t, h, agents.RecordOpts{TaskKey: key, Agent: agent})
				if err := c.Agents.Complete(key, sdk.RunResult{ExitCode: 0}); err != nil {
					t.Fatalf("Agents.Complete(%q) error = %v", key, err)
				}
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
			wantSpawn: true,
			wantRuns:  1,
		},
		{
			name: "missing agent config",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				setGhostRule(t, c)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
		},
		{
			name: "missing task",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
			},
			event: func(string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: "nosuchtask", Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
		},
		{
			name: "spawn attempted for an agent column",
			setup: func(t *testing.T, _ *Host, c sdk.Context, _ string) {
				registerAgent(t, c, agent)
				setRule(t, c, "review", agentRule)
			},
			event: func(key string) pkgevents.Event {
				return pkgevents.Event{Type: pkgevents.TaskMoved, Key: key, Author: "alice", Metadata: map[string]any{"to": "review"}}
			},
			wantSpawn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := TestSetup(t, TestWorkdir)
			c := h.Context(t.Context(), "alice")
			task := addTask(t, c, "Pipeline task", spec, sdk.TaskAddOpts{Author: "alice"})
			if tt.setup != nil {
				tt.setup(t, h, c, task.Key)
			}

			git := &spyGit{}
			store := h.current()
			handler := &pipelineHandler{dir: store.Dir(), agent: newAgentAPI(store, git, true, "", t.Context())}

			e := tt.event(task.Key)
			if err := handler.HandleEvent(t.Context(), e); err != nil {
				t.Fatalf("HandleEvent(%s to %v) error = %v, want nil", e.Type, e.Metadata["to"], err)
			}
			if git.called != tt.wantSpawn {
				t.Errorf("HandleEvent(%s to %v) reached spawn = %v, want %v", e.Type, e.Metadata["to"], git.called, tt.wantSpawn)
			}
			if got := runCount(t, c, task.Key); got != tt.wantRuns {
				t.Errorf("runs for task after HandleEvent = %d, want %d", got, tt.wantRuns)
			}
		})
	}
}

// TestPipelineFailedSpawnKeepsMove moves a task into a column whose
// rule names an agent that is not registered. The pipeline handler
// wired by the host fails to spawn, and the move stands.
func TestPipelineFailedSpawnKeepsMove(t *testing.T) {
	h := TestSetup(t, TestWorkdir)
	c := h.Context(t.Context(), "alice")
	setGhostRule(t, c)
	task := addTask(t, c, "Reviewed", spec, sdk.TaskAddOpts{Author: "alice"})

	if err := c.Tasks.Move(task.Key, "review", "alice"); err != nil {
		t.Fatalf("Move(%q, review) error = %v", task.Key, err)
	}
	if got := readTask(t, c, task.Key).Status; got != "review" {
		t.Errorf("status after move with failed auto-spawn = %q, want %q", got, "review")
	}
	if got := runCount(t, c, task.Key); got != 0 {
		t.Errorf("runs after failed auto-spawn = %d, want 0", got)
	}
}
