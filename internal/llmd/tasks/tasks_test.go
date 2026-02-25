package tasks

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/events"
	intsql "github.com/jpl-au/llmd/internal/sql"
	"github.com/jpl-au/llmd/pkg/model/core"

	"database/sql"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) *Tasks {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := intsql.Exec(db); err != nil {
		t.Fatal(err)
	}

	bus := events.New()
	docs := documents.New(db, bus)
	ents := entities.New(db)
	aud := audit.New(db)

	return New(db, docs, ents, aud)
}

func TestAdd(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	tsk, err := ts.Add(ctx, "Fix auth bug", []byte("## Spec\n\nFix the bug."), AddOptions{
		Origin:   core.Origin{Author: "alice", Source: "test"},
		Priority: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if tsk.Title != "Fix auth bug" {
		t.Errorf("title = %q, want %q", tsk.Title, "Fix auth bug")
	}
	if tsk.Status != "backlog" {
		t.Errorf("status = %q, want %q", tsk.Status, "backlog")
	}
	if tsk.Priority != 1 {
		t.Errorf("priority = %d, want %d", tsk.Priority, 1)
	}
	if tsk.Path != "tasks/fix-auth-bug" {
		t.Errorf("path = %q, want %q", tsk.Path, "tasks/fix-auth-bug")
	}
}

func TestAddCustomStatus(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	tsk, err := ts.Add(ctx, "Urgent fix", nil, AddOptions{
		Origin: core.Origin{Author: "alice", Source: "test"},
		Status: "up-next",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tsk.Status != "up-next" {
		t.Errorf("status = %q, want %q", tsk.Status, "up-next")
	}
}

func TestAddInvalidColumn(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	_, err := ts.Add(ctx, "Bad status", nil, AddOptions{
		Origin: core.Origin{Author: "alice", Source: "test"},
		Status: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for invalid column")
	}
}

func TestMissingTitle(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	_, err := ts.Add(ctx, "", nil, AddOptions{
		Origin: core.Origin{Author: "alice", Source: "test"},
	})
	if err != ErrMissingTitle {
		t.Fatalf("err = %v, want ErrMissingTitle", err)
	}
}

func TestReadAndList(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	t1, _ := ts.Add(ctx, "Task one", nil, AddOptions{Origin: origin})
	t2, _ := ts.Add(ctx, "Task two", nil, AddOptions{Origin: origin})

	// Read by key
	got, err := ts.Read(ctx, t1.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Task one" {
		t.Errorf("title = %q, want %q", got.Title, "Task one")
	}

	// List all
	all, err := ts.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("len = %d, want 2", len(all))
	}

	// Position ordering
	if all[0].Key != t1.Key || all[1].Key != t2.Key {
		t.Error("tasks not ordered by position")
	}
}

func TestMove(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	// Create with spec content so spec gating passes
	tsk, _ := ts.Add(ctx, "Specced task", []byte("# Specced task\n\nReal content here."), AddOptions{Origin: origin})

	if err := ts.Move(ctx, tsk.Key, "up-next", "alice"); err != nil {
		t.Fatal(err)
	}

	got, _ := ts.Read(ctx, tsk.Key)
	if got.Status != "up-next" {
		t.Errorf("status = %q, want %q", got.Status, "up-next")
	}
}

func TestMoveSpecGating(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	// Create without spec content (template heading only)
	tsk, _ := ts.Add(ctx, "No spec", nil, AddOptions{Origin: origin})

	err := ts.Move(ctx, tsk.Key, "up-next", "alice")
	if err != ErrNoSpec {
		t.Fatalf("err = %v, want ErrNoSpec", err)
	}
}

func TestDelete(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "To delete", nil, AddOptions{Origin: origin})

	deleted, err := ts.Delete(ctx, tsk.Key, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Title != "To delete" {
		t.Errorf("title = %q, want %q", deleted.Title, "To delete")
	}

	// Should not be found now
	_, err = ts.Read(ctx, tsk.Key)
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRestore(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "To restore", nil, AddOptions{Origin: origin})
	ts.Delete(ctx, tsk.Key, "alice")

	restored, err := ts.Restore(ctx, tsk.Key, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if restored.Title != "To restore" {
		t.Errorf("title = %q, want %q", restored.Title, "To restore")
	}

	// Should be readable again
	got, err := ts.Read(ctx, tsk.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "To restore" {
		t.Errorf("title = %q, want %q", got.Title, "To restore")
	}
}

func TestFlags(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Flagged", nil, AddOptions{Origin: origin})

	// Add flag
	ts.Set(ctx, tsk.Key, "alice", SetOptions{Flag: "blocked"})
	got, _ := ts.Read(ctx, tsk.Key)
	if got.Flags != "blocked" {
		t.Errorf("flags = %q, want %q", got.Flags, "blocked")
	}

	// Add another
	ts.Set(ctx, tsk.Key, "alice", SetOptions{Flag: "hold"})
	got, _ = ts.Read(ctx, tsk.Key)
	if got.Flags != "blocked,hold" {
		t.Errorf("flags = %q, want %q", got.Flags, "blocked,hold")
	}

	// Remove one
	ts.Set(ctx, tsk.Key, "alice", SetOptions{Unflag: "blocked"})
	got, _ = ts.Read(ctx, tsk.Key)
	if got.Flags != "hold" {
		t.Errorf("flags = %q, want %q", got.Flags, "hold")
	}
}

func TestColumns(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	// Trigger board creation
	ts.Add(ctx, "Trigger", nil, AddOptions{Origin: origin})

	cols, err := ts.Columns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 5 {
		t.Fatalf("len = %d, want 5", len(cols))
	}

	// Add column
	if err := ts.AddColumn(ctx, "testing", "review", "alice"); err != nil {
		t.Fatal(err)
	}
	cols, _ = ts.Columns(ctx)
	if len(cols) != 6 {
		t.Fatalf("len = %d, want 6", len(cols))
	}
	if cols[4] != "testing" {
		t.Errorf("cols[4] = %q, want %q", cols[4], "testing")
	}

	// Remove column
	if err := ts.RemoveColumn(ctx, "testing", "alice"); err != nil {
		t.Fatal(err)
	}
	cols, _ = ts.Columns(ctx)
	if len(cols) != 5 {
		t.Fatalf("len = %d, want 5", len(cols))
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix auth bug", "fix-auth-bug"},
		{"Update API docs", "update-api-docs"},
		{"Hello, World!", "hello-world"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"already-slugged", "already-slugged"},
	}
	for _, tt := range tests {
		got := slug(tt.input)
		if got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPosition(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	t1, _ := ts.Add(ctx, "First", nil, AddOptions{Origin: origin})
	t2, _ := ts.Add(ctx, "Second", nil, AddOptions{Origin: origin})
	t3, _ := ts.Add(ctx, "Third", nil, AddOptions{Origin: origin})

	// Move third to top
	ts.Set(ctx, t3.Key, "alice", SetOptions{Position: new(int)})

	all, _ := ts.List(ctx, ListOptions{})
	if all[0].Key != t3.Key {
		t.Errorf("first task = %q, want %q", all[0].Key, t3.Key)
	}
	if all[1].Key != t1.Key {
		t.Errorf("second task = %q, want %q", all[1].Key, t1.Key)
	}
	if all[2].Key != t2.Key {
		t.Errorf("third task = %q, want %q", all[2].Key, t2.Key)
	}
}

func TestLogSingleTask(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Logged task", []byte("# Logged task\n\nSpec content."), AddOptions{Origin: origin})

	// Move and set to generate audit entries
	ts.Move(ctx, tsk.Key, "up-next", "alice")
	ts.Set(ctx, tsk.Key, "bob", SetOptions{Flag: "blocked"})

	events, err := ts.Log(ctx, tsk.Key, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 3 {
		t.Fatalf("len = %d, want 3", len(events))
	}

	// Verify all three actions are present (order depends on insert ID
	// when timestamps are identical within the same millisecond)
	actions := map[string]bool{}
	for _, e := range events {
		actions[e.Action] = true
		if e.Subject != tsk.Key {
			t.Errorf("subject = %q, want %q", e.Subject, tsk.Key)
		}
	}
	for _, want := range []string{"created", "moved", "flagged"} {
		if !actions[want] {
			t.Errorf("missing action %q in log", want)
		}
	}
}

func TestLogAllTasks(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	t1, _ := ts.Add(ctx, "Task A", nil, AddOptions{Origin: origin})
	t2, _ := ts.Add(ctx, "Task B", nil, AddOptions{Origin: origin})

	// All history, no filter
	events, err := ts.Log(ctx, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len = %d, want 2", len(events))
	}

	// Subjects should be the two task keys (newest first)
	if events[0].Subject != t2.Key {
		t.Errorf("events[0].Subject = %q, want %q", events[0].Subject, t2.Key)
	}
	if events[1].Subject != t1.Key {
		t.Errorf("events[1].Subject = %q, want %q", events[1].Subject, t1.Key)
	}
}

func TestLogLimit(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Many events", []byte("# Many events\n\nSpec."), AddOptions{Origin: origin})
	ts.Move(ctx, tsk.Key, "up-next", "alice")
	ts.Set(ctx, tsk.Key, "alice", SetOptions{Flag: "hold"})

	// Limit to 2
	events, err := ts.Log(ctx, tsk.Key, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len = %d, want 2", len(events))
	}
}

func TestLogNotFound(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	_, err := ts.Log(ctx, "nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestAddWithoutBody(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, err := ts.Add(ctx, "No body task", nil, AddOptions{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	if tsk.Path != "tasks/no-body-task" {
		t.Errorf("path = %q, want %q", tsk.Path, "tasks/no-body-task")
	}

	// Document should NOT exist
	_, err = ts.docs.Read(ctx, tsk.Path)
	if err == nil {
		t.Error("expected document to not exist")
	}
}

func TestAddWithBody(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	body := []byte("## Spec\n\nReal content here.")
	tsk, err := ts.Add(ctx, "With body", body, AddOptions{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}

	// Document should exist with correct content
	doc, err := ts.docs.Read(ctx, tsk.Path)
	if err != nil {
		t.Fatalf("reading document: %v", err)
	}
	if doc.Content != string(body) {
		t.Errorf("content = %q, want %q", doc.Content, string(body))
	}
}

func TestAddWithBranch(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, err := ts.Add(ctx, "Branched task", nil, AddOptions{
		Origin: origin,
		Branch: "feature-auth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tsk.Branch != "feature-auth" {
		t.Errorf("branch = %q, want %q", tsk.Branch, "feature-auth")
	}

	// Read back and verify
	got, err := ts.Read(ctx, tsk.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "feature-auth" {
		t.Errorf("branch after read = %q, want %q", got.Branch, "feature-auth")
	}
}

func TestSetBranch(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Set branch", nil, AddOptions{Origin: origin})

	branch := "feature-xyz"
	if err := ts.Set(ctx, tsk.Key, "alice", SetOptions{Branch: &branch}); err != nil {
		t.Fatal(err)
	}

	got, _ := ts.Read(ctx, tsk.Key)
	if got.Branch != "feature-xyz" {
		t.Errorf("branch = %q, want %q", got.Branch, "feature-xyz")
	}
}

func TestClearBranch(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Clear branch", nil, AddOptions{
		Origin: origin,
		Branch: "old-branch",
	})

	empty := ""
	if err := ts.Set(ctx, tsk.Key, "alice", SetOptions{Branch: &empty}); err != nil {
		t.Fatal(err)
	}

	got, _ := ts.Read(ctx, tsk.Key)
	if got.Branch != "" {
		t.Errorf("branch = %q, want empty", got.Branch)
	}
}
