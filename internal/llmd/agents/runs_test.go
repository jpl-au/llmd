package agents

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/jpl-au/llmd/internal/sqlite/sqlitetest"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/events"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) *Agents {
	t.Helper()
	db := sqlitetest.Open(t, "work.db")

	bus := events.New()
	dbFn := func() *qwr.Manager { return db }
	return New(dbFn, t.TempDir(), bus)
}

// record spawns a running agent for task1.
func record(t *testing.T, a *Agents) {
	t.Helper()
	if _, err := a.Record(t.Context(), RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
}

// recordThen spawns a running agent for task1 and applies finish to it.
func recordThen(finish func(t *testing.T, a *Agents)) func(t *testing.T, a *Agents) {
	return func(t *testing.T, a *Agents) {
		t.Helper()
		record(t, a)
		finish(t, a)
	}
}

func complete(exitCode int) func(t *testing.T, a *Agents) {
	return func(t *testing.T, a *Agents) {
		t.Helper()
		if err := a.Complete(t.Context(), "task1", CompleteOpts{ExitCode: exitCode}); err != nil {
			t.Fatalf("Complete() error = %v", err)
		}
	}
}

func stop(t *testing.T, a *Agents) {
	t.Helper()
	if _, err := a.MarkStopped(t.Context(), "task1", "admin"); err != nil {
		t.Fatalf("MarkStopped() error = %v", err)
	}
}

// eqPtr reports whether two optional values are both nil or both
// present and equal.
func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// TestRecord covers spawning a run: the materialised fields of a new
// run, rejection while a run is active, and acceptance once the
// previous run has reached any terminal state. Successful cases also
// check that RunByTask returns the new run.
func TestRecord(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, a *Agents)
		opts    RecordOpts
		wantErr error
	}{
		{
			name: "first run",
			opts: RecordOpts{
				TaskKey:  "task1",
				Agent:    "claude-code",
				Branch:   "task/fix-bug",
				Worktree: "/tmp/worktree",
				PID:      1234,
				Author:   "dev",
			},
		},
		{
			name:    "rejects double spawn",
			setup:   record,
			opts:    RecordOpts{TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev"},
			wantErr: ErrRunning,
		},
		{
			name:  "after completion",
			setup: recordThen(complete(0)),
			opts:  RecordOpts{TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev"},
		},
		{
			name:  "after failure",
			setup: recordThen(complete(1)),
			opts:  RecordOpts{TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev"},
		},
		{
			name:  "after stop",
			setup: recordThen(stop),
			opts:  RecordOpts{TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev"},
		},
		{
			name:  "different task while running",
			setup: record,
			opts:  RecordOpts{TaskKey: "task2", Agent: "gemini", PID: 2, Author: "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := setup(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, a)
			}

			r, err := a.Record(ctx, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Record(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if r.Key == "" {
				t.Error("Record() returned empty key")
			}
			if r.Status != "running" {
				t.Errorf("Record().Status = %q, want %q", r.Status, "running")
			}
			if r.ExitCode != -1 {
				t.Errorf("Record().ExitCode = %d, want -1", r.ExitCode)
			}
			if r.TaskKey != tt.opts.TaskKey {
				t.Errorf("Record().TaskKey = %q, want %q", r.TaskKey, tt.opts.TaskKey)
			}
			if r.Agent != tt.opts.Agent {
				t.Errorf("Record().Agent = %q, want %q", r.Agent, tt.opts.Agent)
			}
			if r.Branch != tt.opts.Branch {
				t.Errorf("Record().Branch = %q, want %q", r.Branch, tt.opts.Branch)
			}
			if r.Worktree != tt.opts.Worktree {
				t.Errorf("Record().Worktree = %q, want %q", r.Worktree, tt.opts.Worktree)
			}
			if r.PID != tt.opts.PID {
				t.Errorf("Record().PID = %d, want %d", r.PID, tt.opts.PID)
			}

			// RunByTask should return the most recent run.
			got, err := a.RunByTask(ctx, tt.opts.TaskKey)
			if err != nil {
				t.Fatalf("RunByTask(%q) error = %v", tt.opts.TaskKey, err)
			}
			if got.Key != r.Key {
				t.Errorf("RunByTask(%q).Key = %q, want most recent %q", tt.opts.TaskKey, got.Key, r.Key)
			}
			if got.Status != "running" {
				t.Errorf("RunByTask(%q).Status = %q, want %q", tt.opts.TaskKey, got.Status, "running")
			}
		})
	}
}

// TestComplete covers the terminal "completed" and "failed" events,
// the stats they carry, and rejection when no run is active. The
// materialised view is checked through RunByTask.
// TestRecordConcurrent checks that concurrent spawns for one task
// start exactly one run: the running check once sat outside the insert
// transaction, so several could pass it. Run with -race.
func TestRecordConcurrent(t *testing.T) {
	a := setup(t)
	const spawns = 8

	errs := make([]error, spawns)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range spawns {
		wg.Go(func() {
			<-start
			_, errs[i] = a.Record(t.Context(), RecordOpts{TaskKey: "task000001", Agent: "claude", Author: "alice"})
		})
	}
	close(start)
	wg.Wait()

	started := 0
	for i, err := range errs {
		switch {
		case err == nil:
			started++
		case !errors.Is(err, ErrRunning):
			t.Errorf("Record() spawn %d error = %v, want nil or ErrRunning", i, err)
		}
	}
	if started != 1 {
		t.Errorf("Record() started %d runs for one task, want 1", started)
	}
}

func TestComplete(t *testing.T) {
	cost := 0.50
	inTok := 1000
	outTok := 500

	tests := []struct {
		name       string
		setup      func(t *testing.T, a *Agents)
		opts       CompleteOpts
		wantErr    error
		wantStatus string // status after the call, when a run exists
	}{
		{
			name:  "completed with stats",
			setup: record,
			opts: CompleteOpts{
				ExitCode:     0,
				MonetaryCost: &cost,
				InputTokens:  &inTok,
				OutputTokens: &outTok,
				Model:        "claude-opus-4-6",
				SessionID:    "sess-abc-123",
			},
			wantStatus: "completed",
		},
		{
			name:       "failed",
			setup:      record,
			opts:       CompleteOpts{ExitCode: 1},
			wantStatus: "failed",
		},
		{
			name:       "rejects already completed",
			setup:      recordThen(complete(0)),
			opts:       CompleteOpts{ExitCode: 0},
			wantErr:    ErrNotRunning,
			wantStatus: "completed",
		},
		{
			name:       "rejects stopped",
			setup:      recordThen(stop),
			opts:       CompleteOpts{ExitCode: 0},
			wantErr:    ErrNotRunning,
			wantStatus: "stopped",
		},
		{
			name:    "rejects missing run",
			opts:    CompleteOpts{ExitCode: 0},
			wantErr: ErrRunNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := setup(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, a)
			}

			err := a.Complete(ctx, "task1", tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Complete(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
			}
			if tt.wantStatus == "" {
				return
			}

			got, err := a.RunByTask(ctx, "task1")
			if err != nil {
				t.Fatalf("RunByTask() error = %v", err)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("RunByTask().Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if tt.wantErr != nil {
				return
			}
			if got.ExitCode != tt.opts.ExitCode {
				t.Errorf("RunByTask().ExitCode = %d, want %d", got.ExitCode, tt.opts.ExitCode)
			}
			if !eqPtr(got.MonetaryCost, tt.opts.MonetaryCost) {
				t.Errorf("RunByTask().MonetaryCost = %v, want %v", got.MonetaryCost, tt.opts.MonetaryCost)
			}
			if !eqPtr(got.InputTokens, tt.opts.InputTokens) {
				t.Errorf("RunByTask().InputTokens = %v, want %v", got.InputTokens, tt.opts.InputTokens)
			}
			if !eqPtr(got.OutputTokens, tt.opts.OutputTokens) {
				t.Errorf("RunByTask().OutputTokens = %v, want %v", got.OutputTokens, tt.opts.OutputTokens)
			}
			if got.Model != tt.opts.Model {
				t.Errorf("RunByTask().Model = %q, want %q", got.Model, tt.opts.Model)
			}
			if got.SessionID != tt.opts.SessionID {
				t.Errorf("RunByTask().SessionID = %q, want %q", got.SessionID, tt.opts.SessionID)
			}
			if got.StoppedAt == 0 {
				t.Error("RunByTask().StoppedAt = 0, want non-zero after completion")
			}
		})
	}
}

// TestMarkStopped covers manual termination of a running agent and
// rejection when the run is not running.
func TestMarkStopped(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, a *Agents)
		wantErr error
	}{
		{"running", record, nil},
		{"rejects completed", recordThen(complete(0)), ErrNotRunning},
		{"rejects missing run", nil, ErrRunNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := setup(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, a)
			}

			stopped, err := a.MarkStopped(ctx, "task1", "admin")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("MarkStopped() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if stopped.Status != "stopped" {
				t.Errorf("MarkStopped().Status = %q, want %q", stopped.Status, "stopped")
			}

			// Verify via RunByTask.
			got, err := a.RunByTask(ctx, "task1")
			if err != nil {
				t.Fatalf("RunByTask() error = %v", err)
			}
			if got.Status != "stopped" {
				t.Errorf("RunByTask().Status = %q, want %q", got.Status, "stopped")
			}
		})
	}
}

// TestList covers every ListOpts filter against two runs: task1 by
// claude-code (completed) and task2 by gemini (running). Results are
// compared as sorted agent names.
func TestList(t *testing.T) {
	tests := []struct {
		name string
		opts ListOpts
		want []string // sorted agents
	}{
		{"all", ListOpts{}, []string{"claude-code", "gemini"}},
		{"by status running", ListOpts{Status: "running"}, []string{"gemini"}},
		{"by status completed", ListOpts{Status: "completed"}, []string{"claude-code"}},
		{"by agent", ListOpts{Agent: "claude-code"}, []string{"claude-code"}},
		{"by task", ListOpts{TaskKey: "task2"}, []string{"gemini"}},
		{"no match", ListOpts{TaskKey: "task3"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := setup(t)
			ctx := t.Context()

			record(t, a)
			if _, err := a.Record(ctx, RecordOpts{TaskKey: "task2", Agent: "gemini", PID: 2, Author: "dev"}); err != nil {
				t.Fatalf("Record(task2) error = %v", err)
			}
			complete(0)(t, a)

			runs, err := a.List(ctx, tt.opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tt.opts, err)
			}
			var got []string
			for _, r := range runs {
				got = append(got, r.Agent)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%+v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
	}
}

func TestRunByTaskNotFound(t *testing.T) {
	a := setup(t)

	_, err := a.RunByTask(t.Context(), "nonexistent")
	if !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("RunByTask(%q) error = %v, want ErrRunNotFound", "nonexistent", err)
	}
}

func TestInsertOnlyAuditTrail(t *testing.T) {
	a := setup(t)
	ctx := t.Context()

	// Verify that the event log captures the full lifecycle.
	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := a.Complete(ctx, "task1", CompleteOpts{
		ExitCode:  0,
		SessionID: "sess-123",
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Query the events table directly to verify insert-only behaviour.
	rows, err := a.db.Query(`
		SELECT event, session_id FROM agent_events
		WHERE run_key = (SELECT key FROM agent_runs WHERE task_key = 'task1')
		ORDER BY created_at
	`).Read()
	if err != nil {
		t.Fatalf("querying events: %v", err)
	}
	defer rows.Close()

	type evt struct {
		event     string
		sessionID *string
	}
	var got []evt
	for rows.Next() {
		var e evt
		var sid *string
		if err := rows.Scan(&e.event, &sid); err != nil {
			t.Fatalf("scanning event: %v", err)
		}
		e.sessionID = sid
		got = append(got, e)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].event != "spawned" {
		t.Errorf("first event = %q, want %q", got[0].event, "spawned")
	}
	if got[1].event != "completed" {
		t.Errorf("second event = %q, want %q", got[1].event, "completed")
	}
	if got[1].sessionID == nil || *got[1].sessionID != "sess-123" {
		t.Errorf("completion session_id = %v, want %q", got[1].sessionID, "sess-123")
	}
}
