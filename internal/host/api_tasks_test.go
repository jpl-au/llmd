package host

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func TestTasksAdd(t *testing.T) {
	testHost(t)

	task, err := sdk.Tasks.Add("Fix the bug", nil, sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.Title != "Fix the bug" {
		t.Errorf("title = %q, want %q", task.Title, "Fix the bug")
	}
	if task.Status != "backlog" {
		t.Errorf("status = %q, want %q", task.Status, "backlog")
	}
	if task.Key == "" {
		t.Error("key is empty")
	}
	if task.Path == "" {
		t.Error("path is empty")
	}
}

func TestTasksAddWithBody(t *testing.T) {
	testHost(t)

	body := []byte("## Spec\n\nFix the auth tokens.")
	task, err := sdk.Tasks.Add("Auth fix", body, sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// The spec document should exist
	ok, _ := sdk.Documents.Exists(task.Path)
	if !ok {
		t.Error("spec document does not exist")
	}

	content, _ := sdk.Documents.Read(task.Path, 0)
	if string(content) != string(body) {
		t.Errorf("spec content = %q, want %q", content, body)
	}
}

func TestTasksAddWithColumn(t *testing.T) {
	testHost(t)

	task, err := sdk.Tasks.Add("Urgent", nil, sdk.TaskAddOpts{
		Author: "alice",
		Status: "up-next",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.Status != "up-next" {
		t.Errorf("status = %q, want %q", task.Status, "up-next")
	}
}

func TestTasksAddWithPriority(t *testing.T) {
	testHost(t)

	task, err := sdk.Tasks.Add("High pri", nil, sdk.TaskAddOpts{
		Author:   "alice",
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want 1", task.Priority)
	}
}

func TestTasksAddWithPath(t *testing.T) {
	testHost(t)

	// Create an existing document first
	sdk.Documents.Write("specs/auth", []byte("# Auth Spec"), "alice", "")

	task, err := sdk.Tasks.Add("Auth work", nil, sdk.TaskAddOpts{
		Author: "alice",
		Path:   "specs/auth",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.Path != "specs/auth" {
		t.Errorf("path = %q, want %q", task.Path, "specs/auth")
	}
}

func TestTasksRead(t *testing.T) {
	testHost(t)

	created, _ := sdk.Tasks.Add("Read me", nil, sdk.TaskAddOpts{Author: "alice"})

	task, err := sdk.Tasks.Read(created.Key)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if task.Title != "Read me" {
		t.Errorf("title = %q, want %q", task.Title, "Read me")
	}
}

func TestTasksReadNotFound(t *testing.T) {
	testHost(t)

	_, err := sdk.Tasks.Read("nonexistent")
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Read error = %v, want sdk.ErrNotFound", err)
	}
}

func TestTasksList(t *testing.T) {
	testHost(t)

	sdk.Tasks.Add("One", nil, sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Add("Two", nil, sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Add("Three", nil, sdk.TaskAddOpts{Author: "alice", Status: "up-next"})

	tasks, err := sdk.Tasks.List(sdk.TaskListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("List returned %d tasks, want 3", len(tasks))
	}
}

func TestTasksListFilterByColumn(t *testing.T) {
	testHost(t)

	sdk.Tasks.Add("Backlog", nil, sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Add("Next", nil, sdk.TaskAddOpts{Author: "alice", Status: "up-next"})

	tasks, _ := sdk.Tasks.List(sdk.TaskListOpts{Status: "up-next"})
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Title != "Next" {
		t.Errorf("title = %q, want %q", tasks[0].Title, "Next")
	}
}

func TestTasksListFilterByAssignee(t *testing.T) {
	testHost(t)

	sdk.Tasks.Add("Alice task", nil, sdk.TaskAddOpts{Author: "alice", AssignedTo: "alice"})
	sdk.Tasks.Add("Bob task", nil, sdk.TaskAddOpts{Author: "bob", AssignedTo: "bob"})

	tasks, _ := sdk.Tasks.List(sdk.TaskListOpts{AssignedTo: "alice"})
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
}

func TestTasksMove(t *testing.T) {
	testHost(t)

	// Create task with a multi-line spec so it can leave backlog.
	// hasSpec strips the first line (heading) and checks for content after it.
	task, _ := sdk.Tasks.Add("Movable", []byte("# Spec\n\nDo the thing."), sdk.TaskAddOpts{Author: "alice"})

	if err := sdk.Tasks.Move(task.Key, "in-progress", "alice"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "in-progress" {
		t.Errorf("status = %q, want %q", updated.Status, "in-progress")
	}
}

func TestTasksMoveNoSpec(t *testing.T) {
	testHost(t)

	// Task without body — no spec document
	task, _ := sdk.Tasks.Add("No spec", nil, sdk.TaskAddOpts{Author: "alice"})

	err := sdk.Tasks.Move(task.Key, "in-progress", "alice")
	if err == nil {
		t.Fatal("Move should fail for task without spec")
	}
	if !errors.Is(err, sdk.ErrNoSpec) {
		t.Errorf("error = %v, want ErrNoSpec", err)
	}
}

func TestTasksSet(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Original", nil, sdk.TaskAddOpts{Author: "alice"})

	newTitle := "Updated"
	newPri := 1
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{
		Title:    &newTitle,
		Priority: &newPri,
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Title != "Updated" {
		t.Errorf("title = %q, want %q", updated.Title, "Updated")
	}
	if updated.Priority != 1 {
		t.Errorf("priority = %d, want 1", updated.Priority)
	}
}

func TestTasksSetFlags(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Flagged", nil, sdk.TaskAddOpts{Author: "alice"})

	sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Flag: "blocked"})
	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Flags != "blocked" {
		t.Errorf("flags = %q, want %q", updated.Flags, "blocked")
	}

	sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Unflag: "blocked"})
	updated, _ = sdk.Tasks.Read(task.Key)
	if updated.Flags != "" {
		t.Errorf("flags = %q, want empty after unflag", updated.Flags)
	}
}

func TestTasksDeleteRestore(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Deletable", nil, sdk.TaskAddOpts{Author: "alice"})

	deleted, err := sdk.Tasks.Delete(task.Key, "alice")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if deleted.Title != "Deletable" {
		t.Errorf("deleted title = %q, want %q", deleted.Title, "Deletable")
	}

	// Should not appear in list
	tasks, _ := sdk.Tasks.List(sdk.TaskListOpts{})
	if len(tasks) != 0 {
		t.Errorf("List after delete: got %d, want 0", len(tasks))
	}

	restored, err := sdk.Tasks.Restore(task.Key, "alice")
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.Title != "Deletable" {
		t.Errorf("restored title = %q, want %q", restored.Title, "Deletable")
	}

	tasks, _ = sdk.Tasks.List(sdk.TaskListOpts{})
	if len(tasks) != 1 {
		t.Errorf("List after restore: got %d, want 1", len(tasks))
	}
}

func TestTasksColumns(t *testing.T) {
	testHost(t)

	cols, err := sdk.Tasks.Columns()
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}

	expected := []string{"backlog", "up-next", "in-progress", "review", "done"}
	if len(cols) != len(expected) {
		t.Fatalf("got %d columns, want %d", len(cols), len(expected))
	}
	for i, col := range cols {
		if col != expected[i] {
			t.Errorf("column[%d] = %q, want %q", i, col, expected[i])
		}
	}
}

func TestTasksAddColumn(t *testing.T) {
	testHost(t)

	if err := sdk.Tasks.AddColumn("testing", "in-progress", "alice"); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}

	cols, _ := sdk.Tasks.Columns()
	found := false
	for i, c := range cols {
		if c == "testing" {
			found = true
			if i == 0 || cols[i-1] != "in-progress" {
				t.Errorf("testing column not after in-progress")
			}
		}
	}
	if !found {
		t.Error("testing column not found")
	}
}

func TestTasksRemoveColumn(t *testing.T) {
	testHost(t)

	sdk.Tasks.AddColumn("temp", "", "alice")

	if err := sdk.Tasks.RemoveColumn("temp", "alice"); err != nil {
		t.Fatalf("RemoveColumn: %v", err)
	}

	cols, _ := sdk.Tasks.Columns()
	for _, c := range cols {
		if c == "temp" {
			t.Error("temp column still exists after RemoveColumn")
		}
	}
}

func TestTasksMoveColumn(t *testing.T) {
	testHost(t)

	sdk.Tasks.AddColumn("qa", "", "alice")

	if err := sdk.Tasks.MoveColumn("qa", "review", "alice"); err != nil {
		t.Fatalf("MoveColumn: %v", err)
	}

	cols, _ := sdk.Tasks.Columns()
	for i, c := range cols {
		if c == "qa" && (i == 0 || cols[i-1] != "review") {
			t.Errorf("qa not after review")
		}
	}
}

func TestTasksLog(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Logged", []byte("# Spec\n\nDetails here."), sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Move(task.Key, "in-progress", "alice")

	events, err := sdk.Tasks.Log(task.Key, 0)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(events) == 0 {
		t.Error("Log returned no events")
	}

	// Should have a move event
	found := false
	for _, e := range events {
		if e.Action == "moved" {
			found = true
		}
	}
	if !found {
		t.Error("no move event found in log")
	}
}

func TestTasksLogLimit(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Many events", []byte("# Spec\n\nLots of changes."), sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Move(task.Key, "up-next", "alice")
	sdk.Tasks.Move(task.Key, "in-progress", "alice")
	sdk.Tasks.Move(task.Key, "review", "alice")

	events, _ := sdk.Tasks.Log(task.Key, 2)
	if len(events) > 2 {
		t.Errorf("Log limit 2 returned %d events", len(events))
	}
}

func TestTasksListFilterByPriority(t *testing.T) {
	testHost(t)

	sdk.Tasks.Add("Normal", nil, sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Add("Urgent", nil, sdk.TaskAddOpts{Author: "alice", Priority: 1})
	sdk.Tasks.Add("Critical", nil, sdk.TaskAddOpts{Author: "alice", Priority: 2})

	tasks, err := sdk.Tasks.List(sdk.TaskListOpts{Priority: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Title != "Urgent" {
		t.Errorf("title = %q, want %q", tasks[0].Title, "Urgent")
	}
}

func TestTasksListEmpty(t *testing.T) {
	testHost(t)

	tasks, err := sdk.Tasks.List(sdk.TaskListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("List on empty store: got %d, want 0", len(tasks))
	}
}

func TestTasksSetAssignee(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Assign me", nil, sdk.TaskAddOpts{Author: "alice"})

	assignee := "bob"
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{
		AssignedTo: &assignee,
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.AssignedTo != "bob" {
		t.Errorf("assigned_to = %q, want %q", updated.AssignedTo, "bob")
	}
}

func TestTasksMoveAcrossColumns(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Journey", []byte("# Spec\n\nFull lifecycle."), sdk.TaskAddOpts{Author: "alice"})

	// Move through the whole board
	for _, col := range []string{"up-next", "in-progress", "review", "done"} {
		if err := sdk.Tasks.Move(task.Key, col, "alice"); err != nil {
			t.Fatalf("Move to %s: %v", col, err)
		}
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "done" {
		t.Errorf("status = %q, want %q", updated.Status, "done")
	}
}

func TestTasksDeleteNotFound(t *testing.T) {
	testHost(t)

	_, err := sdk.Tasks.Delete("nonexistent", "alice")
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Delete error = %v, want sdk.ErrNotFound", err)
	}
}

func TestTasksAddWithAssignee(t *testing.T) {
	testHost(t)

	task, err := sdk.Tasks.Add("Assigned", nil, sdk.TaskAddOpts{
		Author:     "alice",
		AssignedTo: "bob",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.AssignedTo != "bob" {
		t.Errorf("assigned_to = %q, want %q", task.AssignedTo, "bob")
	}
}

func TestTasksAddWithBranch(t *testing.T) {
	testHost(t)

	task, err := sdk.Tasks.Add("Branched", nil, sdk.TaskAddOpts{
		Author: "alice",
		Branch: "feature-login",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if task.Branch != "feature-login" {
		t.Errorf("branch = %q, want %q", task.Branch, "feature-login")
	}

	// Read back
	got, _ := sdk.Tasks.Read(task.Key)
	if got.Branch != "feature-login" {
		t.Errorf("branch after read = %q, want %q", got.Branch, "feature-login")
	}
}

func TestTasksSetBranch(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Set branch", nil, sdk.TaskAddOpts{Author: "alice"})

	branch := "feature-new"
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{
		Branch: &branch,
	}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Branch != "feature-new" {
		t.Errorf("branch = %q, want %q", updated.Branch, "feature-new")
	}
}

func TestTasksLogEventDetails(t *testing.T) {
	testHost(t)

	task, _ := sdk.Tasks.Add("Detailed", []byte("# Spec\n\nLog details."), sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Move(task.Key, "in-progress", "alice")

	events, _ := sdk.Tasks.Log(task.Key, 0)

	for _, e := range events {
		if e.Action == "moved" {
			if e.OldValue == "" {
				t.Error("move event missing OldValue")
			}
			if e.NewValue == "" {
				t.Error("move event missing NewValue")
			}
			if e.Actor == "" {
				t.Error("move event missing Actor")
			}
			return
		}
	}
	t.Error("no move event found")
}
