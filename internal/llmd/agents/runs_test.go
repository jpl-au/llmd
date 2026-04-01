package agents

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/profile"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) *Agents {
	t.Helper()
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	db, err := qwr.New("file::memory:?cache=shared").Reader(rp).Writer(wp).Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	bus := events.New()
	dbFn := func() *qwr.Manager { return db }
	return New(dbFn, t.TempDir(), bus)
}

func TestRecord(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	r, err := a.Record(ctx, RecordOpts{
		TaskKey:  "task1",
		Agent:    "claude-code",
		Branch:   "task/fix-bug",
		Worktree: "/tmp/worktree",
		PID:      1234,
		Author:   "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if r.Key == "" {
		t.Error("expected non-empty key")
	}
	if r.Status != "running" {
		t.Errorf("status = %q, want %q", r.Status, "running")
	}
	if r.ExitCode != -1 {
		t.Errorf("exit_code = %d, want -1", r.ExitCode)
	}
	if r.Agent != "claude-code" {
		t.Errorf("agent = %q, want %q", r.Agent, "claude-code")
	}
}

func TestRecordRejectsDoubleSpawn(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("first Record() error = %v", err)
	}

	_, err = a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev",
	})
	if err == nil {
		t.Fatal("second Record() should fail for same task")
	}
}

func TestComplete(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	r, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	cost := 0.50
	inTok := 1000
	outTok := 500
	err = a.Complete(ctx, "task1", CompleteOpts{
		ExitCode:     0,
		MonetaryCost: &cost,
		InputTokens:  &inTok,
		OutputTokens: &outTok,
		Model:        "claude-opus-4-6",
		SessionID:    "sess-abc-123",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Fetch the run and verify the materialised view.
	got, err := a.RunByTask(ctx, "task1")
	if err != nil {
		t.Fatalf("RunByTask() error = %v", err)
	}
	if got.Key != r.Key {
		t.Errorf("key = %q, want %q", got.Key, r.Key)
	}
	if got.Status != "completed" {
		t.Errorf("status = %q, want %q", got.Status, "completed")
	}
	if got.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", got.ExitCode)
	}
	if got.MonetaryCost == nil || *got.MonetaryCost != 0.50 {
		t.Errorf("monetary_cost = %v, want 0.50", got.MonetaryCost)
	}
	if got.InputTokens == nil || *got.InputTokens != 1000 {
		t.Errorf("input_tokens = %v, want 1000", got.InputTokens)
	}
	if got.OutputTokens == nil || *got.OutputTokens != 500 {
		t.Errorf("output_tokens = %v, want 500", got.OutputTokens)
	}
	if got.Model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", got.Model, "claude-opus-4-6")
	}
	if got.SessionID != "sess-abc-123" {
		t.Errorf("session_id = %q, want %q", got.SessionID, "sess-abc-123")
	}
	if got.StoppedAt == 0 {
		t.Error("stopped_at should be non-zero after completion")
	}
}

func TestCompleteFailed(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	err = a.Complete(ctx, "task1", CompleteOpts{ExitCode: 1})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	got, err := a.RunByTask(ctx, "task1")
	if err != nil {
		t.Fatalf("RunByTask() error = %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("status = %q, want %q", got.Status, "failed")
	}
	if got.ExitCode != 1 {
		t.Errorf("exit_code = %d, want 1", got.ExitCode)
	}
}

func TestCompleteRejectsNonRunning(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := a.Complete(ctx, "task1", CompleteOpts{ExitCode: 0}); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}

	// Second complete should fail - already completed.
	if err := a.Complete(ctx, "task1", CompleteOpts{ExitCode: 0}); err == nil {
		t.Fatal("second Complete() should fail on non-running run")
	}
}

func TestMarkStopped(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	stopped, err := a.MarkStopped(ctx, "task1", "admin")
	if err != nil {
		t.Fatalf("MarkStopped() error = %v", err)
	}
	if stopped.Status != "stopped" {
		t.Errorf("status = %q, want %q", stopped.Status, "stopped")
	}

	// Verify via RunByTask.
	got, err := a.RunByTask(ctx, "task1")
	if err != nil {
		t.Fatalf("RunByTask() error = %v", err)
	}
	if got.Status != "stopped" {
		t.Errorf("status = %q, want %q", got.Status, "stopped")
	}
}

func TestSpawnAfterCompletion(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	// First run completes.
	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("first Record() error = %v", err)
	}
	if err := a.Complete(ctx, "task1", CompleteOpts{ExitCode: 0}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Second spawn should succeed - previous run is terminated.
	r2, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 2, Author: "dev",
	})
	if err != nil {
		t.Fatalf("second Record() error = %v", err)
	}

	// RunByTask should return the most recent run.
	got, err := a.RunByTask(ctx, "task1")
	if err != nil {
		t.Fatalf("RunByTask() error = %v", err)
	}
	if got.Key != r2.Key {
		t.Errorf("RunByTask returned key %q, want most recent %q", got.Key, r2.Key)
	}
	if got.Status != "running" {
		t.Errorf("status = %q, want %q", got.Status, "running")
	}
}

func TestList(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	// Create two runs for different tasks.
	_, err := a.Record(ctx, RecordOpts{
		TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record task1 error = %v", err)
	}

	_, err = a.Record(ctx, RecordOpts{
		TaskKey: "task2", Agent: "gemini", PID: 2, Author: "dev",
	})
	if err != nil {
		t.Fatalf("Record task2 error = %v", err)
	}

	// Complete one.
	if err := a.Complete(ctx, "task1", CompleteOpts{ExitCode: 0}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// List all.
	all, err := a.List(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List() returned %d runs, want 2", len(all))
	}

	// Filter by status.
	running, err := a.List(ctx, ListOpts{Status: "running"})
	if err != nil {
		t.Fatalf("List(running) error = %v", err)
	}
	if len(running) != 1 {
		t.Fatalf("List(running) returned %d runs, want 1", len(running))
	}
	if running[0].Agent != "gemini" {
		t.Errorf("running agent = %q, want %q", running[0].Agent, "gemini")
	}

	// Filter by agent.
	claude, err := a.List(ctx, ListOpts{Agent: "claude-code"})
	if err != nil {
		t.Fatalf("List(claude) error = %v", err)
	}
	if len(claude) != 1 {
		t.Fatalf("List(claude) returned %d runs, want 1", len(claude))
	}

	// Filter by task.
	byTask, err := a.List(ctx, ListOpts{TaskKey: "task2"})
	if err != nil {
		t.Fatalf("List(task2) error = %v", err)
	}
	if len(byTask) != 1 {
		t.Fatalf("List(task2) returned %d runs, want 1", len(byTask))
	}
}

func TestRunByTaskNotFound(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

	_, err := a.RunByTask(ctx, "nonexistent")
	if err == nil {
		t.Fatal("RunByTask() should fail for nonexistent task")
	}
}

func TestInsertOnlyAuditTrail(t *testing.T) {
	a := setup(t)
	ctx := context.Background()

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
