package audits

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAdd(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	aud, err := store.Add(ctx, AddOptions{
		Target:   "docs/api",
		Content:  "Needs error handling.",
		Author:   "gemini",
		Assignee: "claude-code",
	})
	if err != nil {
		t.Fatal(err)
	}

	if aud.ID == "" {
		t.Error("expected non-empty ID")
	}
	if aud.TargetType != "document" {
		t.Errorf("target_type = %q, want document", aud.TargetType)
	}
	if aud.Status != "pending" {
		t.Errorf("status = %q, want pending", aud.Status)
	}
	if aud.Assignee != "claude-code" {
		t.Errorf("assignee = %q, want claude-code", aud.Assignee)
	}
	if aud.ParentID != "" {
		t.Errorf("parent_id = %q, want empty", aud.ParentID)
	}
}

func TestAddMissingAuthor(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	_, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "test"})
	if err != ErrMissingAuthor {
		t.Errorf("err = %v, want ErrMissingAuthor", err)
	}
}

func TestAddMissingTarget(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	_, err := store.Add(ctx, AddOptions{Author: "gemini", Content: "test"})
	if err != ErrMissingTarget {
		t.Errorf("err = %v, want ErrMissingTarget", err)
	}
}

func TestInferTargetType(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"docs/api", "document"},
		{"notes/meeting", "document"},
		{"0mmofq4de", "task"},      // valid 9-char base36
		{"000000000", "task"},      // all zeros
		{"docs", "document"},       // too short for task key? no, 4 chars
		{"abcdefghi", "task"},      // 9-char lowercase alpha
		{"ABCDEFGHI", "document"},  // uppercase = not valid base36
		{"abc", "document"},        // too short
		{"abcdefghij", "document"}, // too long
	}

	for _, tt := range tests {
		got := inferTargetType(tt.target)
		if got != tt.want {
			t.Errorf("inferTargetType(%q) = %q, want %q", tt.target, got, tt.want)
		}
	}
}

func TestReply(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	parent, err := store.Add(ctx, AddOptions{
		Target:   "docs/api",
		Content:  "Needs error handling.",
		Author:   "gemini",
		Assignee: "claude-code",
	})
	if err != nil {
		t.Fatal(err)
	}

	reply, err := store.Reply(ctx, parent.ID, AddOptions{
		Content: "Fixed.",
		Author:  "claude-code",
		Status:  "approved",
	})
	if err != nil {
		t.Fatal(err)
	}

	if reply.ParentID != parent.ID {
		t.Errorf("parent_id = %q, want %q", reply.ParentID, parent.ID)
	}
	if reply.Target != parent.Target {
		t.Errorf("target = %q, want %q", reply.Target, parent.Target)
	}
	if reply.Status != "approved" {
		t.Errorf("status = %q, want approved", reply.Status)
	}
	// Assignee inherited from parent.
	if reply.Assignee != "claude-code" {
		t.Errorf("assignee = %q, want claude-code (inherited)", reply.Assignee)
	}
}

func TestReplyReassign(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	parent, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini", Assignee: "claude-code",
	})

	// Reply reassigns to gemini.
	reply, err := store.Reply(ctx, parent.ID, AddOptions{
		Content: "Done, please check.", Author: "claude-code", Assignee: "gemini",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply.Assignee != "gemini" {
		t.Errorf("assignee = %q, want gemini", reply.Assignee)
	}
}

func TestReplyFlattensToTopLevel(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	top, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
	})
	reply1, _ := store.Reply(ctx, top.ID, AddOptions{
		Content: "First reply.", Author: "claude-code",
	})

	// Reply to a reply should resolve to the top-level parent.
	reply2, err := store.Reply(ctx, reply1.ID, AddOptions{
		Content: "Second reply.", Author: "gemini",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply2.ParentID != top.ID {
		t.Errorf("parent_id = %q, want %q (top-level)", reply2.ParentID, top.ID)
	}
}

func TestRead(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	created, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Test.", Author: "gemini",
	})

	read, err := store.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != created.ID {
		t.Errorf("ID = %q, want %q", read.ID, created.ID)
	}
	if read.Content != "Test." {
		t.Errorf("content = %q, want %q", read.Content, "Test.")
	}
}

func TestReadNotFound(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	// Ensure table exists.
	store.Add(ctx, AddOptions{
		Target: "docs/x", Content: "x", Author: "a",
	})

	_, err := store.Read(ctx, "aud_nonexistent")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestThread(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	top, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
	})
	store.Reply(ctx, top.ID, AddOptions{
		Content: "Fixed.", Author: "claude-code", Status: "approved",
	})

	thread, err := store.Thread(ctx, top.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 2 {
		t.Fatalf("len(thread) = %d, want 2", len(thread))
	}
	if thread[0].ID != top.ID {
		t.Error("first entry should be the top-level audit")
	}
	if thread[1].ParentID != top.ID {
		t.Error("second entry should be a reply to top-level")
	}
}

func TestList(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	store.Add(ctx, AddOptions{Target: "docs/api", Content: "A.", Author: "gemini", Assignee: "claude-code"})
	store.Add(ctx, AddOptions{Target: "docs/auth", Content: "B.", Author: "claude-code", Assignee: "gemini"})
	store.Add(ctx, AddOptions{Target: "docs/api", Content: "C.", Author: "gemini", Assignee: "claude-code"})

	// All audits.
	all, err := store.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("len(all) = %d, want 3", len(all))
	}

	// Filter by target.
	byTarget, _ := store.List(ctx, ListOptions{Target: "docs/api"})
	if len(byTarget) != 2 {
		t.Errorf("len(byTarget) = %d, want 2", len(byTarget))
	}

	// Filter by creator.
	byAuthor, _ := store.List(ctx, ListOptions{ByAuthor: "gemini"})
	if len(byAuthor) != 2 {
		t.Errorf("len(byAuthor) = %d, want 2", len(byAuthor))
	}

	// Filter by assignee.
	byAssignee, _ := store.List(ctx, ListOptions{Assignee: "claude-code"})
	if len(byAssignee) != 2 {
		t.Errorf("len(byAssignee) = %d, want 2", len(byAssignee))
	}
}

func TestListPending(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	aud1, _ := store.Add(ctx, AddOptions{Target: "docs/api", Content: "A.", Author: "gemini"})
	store.Add(ctx, AddOptions{Target: "docs/auth", Content: "B.", Author: "gemini", Status: "approved"})
	store.Add(ctx, AddOptions{Target: "docs/config", Content: "C.", Author: "gemini", Status: "needs-work"})

	// Resolve aud1 — its effective status should be "approved".
	store.Resolve(ctx, aud1.ID, "claude-code")

	pending, err := store.List(ctx, ListOptions{Pending: true})
	if err != nil {
		t.Fatal(err)
	}
	// Only docs/config should remain pending (needs-work).
	if len(pending) != 1 {
		t.Errorf("len(pending) = %d, want 1", len(pending))
	}
}

func TestResolve(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	top, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
	})

	resolved, err := store.Resolve(ctx, top.ID, "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != "approved" {
		t.Errorf("status = %q, want approved", resolved.Status)
	}
	if resolved.ParentID != top.ID {
		t.Errorf("parent_id = %q, want %q", resolved.ParentID, top.ID)
	}
	if resolved.Content != "" {
		t.Errorf("content = %q, want empty", resolved.Content)
	}
}

func TestDelete(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	aud, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
	})

	err := store.Delete(ctx, aud.ID, "gemini")
	if err != nil {
		t.Fatal(err)
	}

	// Should not be readable after delete.
	_, err = store.Read(ctx, aud.ID)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStatus(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	// Gemini creates audit assigned to claude-code.
	store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Needs work.", Author: "gemini",
		Assignee: "claude-code", Status: "needs-work",
	})

	// Claude-code creates audit assigned to gemini.
	store.Add(ctx, AddOptions{
		Target: "docs/auth", Content: "LGTM.", Author: "claude-code",
		Assignee: "gemini", Status: "pending",
	})

	// Unassigned audit — should not appear in anyone's status.
	store.Add(ctx, AddOptions{
		Target: "docs/config", Content: "FYI.", Author: "gemini",
		Status: "pending",
	})

	// Claude-code's inbox: should see the audit assigned to them.
	result, err := store.Status(ctx, "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Total != 1 {
		t.Errorf("total = %d, want 1", result.Summary.Total)
	}
	if result.Summary.NeedsWork != 1 {
		t.Errorf("needs_work = %d, want 1", result.Summary.NeedsWork)
	}

	// Gemini's inbox: should see the audit assigned to them.
	result2, _ := store.Status(ctx, "gemini")
	if result2.Summary.Total != 1 {
		t.Errorf("total = %d, want 1", result2.Summary.Total)
	}
	if result2.Summary.Pending != 1 {
		t.Errorf("pending = %d, want 1", result2.Summary.Pending)
	}
}

func TestStatusReassign(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	// Gemini creates audit assigned to claude-code.
	aud, _ := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
		Assignee: "claude-code",
	})

	// Claude-code replies, reassigning to gemini.
	store.Reply(ctx, aud.ID, AddOptions{
		Content: "Done, your turn.", Author: "claude-code", Assignee: "gemini",
	})

	// Claude-code's inbox: should be empty (reassigned away).
	result, _ := store.Status(ctx, "claude-code")
	if result.Summary.Total != 0 {
		t.Errorf("claude-code total = %d, want 0", result.Summary.Total)
	}

	// Gemini's inbox: should see it now.
	result2, _ := store.Status(ctx, "gemini")
	if result2.Summary.Total != 1 {
		t.Errorf("gemini total = %d, want 1", result2.Summary.Total)
	}
}
