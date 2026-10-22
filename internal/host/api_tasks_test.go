package host

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// spec is a task body with content beyond the title heading, so the
// task can leave the backlog (see hasSpec in internal/llmd/tasks).
var spec = []byte("# Spec\n\nDo the thing.")

// addTask creates a task and fails the test on error.
func addTask(t *testing.T, c sdk.Context, title string, body []byte, opts sdk.TaskAddOpts) *sdk.Task {
	t.Helper()
	task, err := c.Tasks.Add(title, body, opts)
	if err != nil {
		t.Fatalf("Add(%q) error = %v", title, err)
	}
	return task
}

// moveTask moves a task and fails the test on error.
func moveTask(t *testing.T, c sdk.Context, key, column string) {
	t.Helper()
	if err := c.Tasks.Move(key, column, "alice"); err != nil {
		t.Fatalf("Move(%q, %q) error = %v", key, column, err)
	}
}

// readTask reads a task and fails the test on error.
func readTask(t *testing.T, c sdk.Context, key string) *sdk.Task {
	t.Helper()
	task, err := c.Tasks.Read(key)
	if err != nil {
		t.Fatalf("Read(%q) error = %v", key, err)
	}
	return task
}

func ptr[T any](v T) *T { return &v }

func TestBranchSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Fix auth tokens", "fix-auth-tokens"},
		{"Add API endpoint", "add-api-endpoint"},
		{"Hello, World!", "hello-world"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPER CASE", "upper-case"},
		{"already-slugged", "already-slugged"},
		{"special!@#chars", "special-chars"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := branchSlug(tt.title)
			if got != tt.want {
				t.Errorf("branchSlug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestTasksStart(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		opts       sdk.StartOpts
		wantErr    error
		wantStatus string
	}{
		{"default column", spec, sdk.StartOpts{}, nil, "in-progress"},
		{"custom column", spec, sdk.StartOpts{Column: "up-next"}, nil, "up-next"},
		{"no spec", nil, sdk.StartOpts{}, sdk.ErrNoSpec, "backlog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Start me", tt.body, sdk.TaskAddOpts{Author: "alice"})
			updated, err := c.Tasks.Start(task.Key, "alice", tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Start(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
			}
			if err != nil {
				updated = readTask(t, c, task.Key)
			}
			if updated.Status != tt.wantStatus {
				t.Errorf("Start(%+v) status = %q, want %q", tt.opts, updated.Status, tt.wantStatus)
			}
		})
	}
}

func TestTasksFinish(t *testing.T) {
	tests := []struct {
		name       string
		opts       sdk.FinishOpts
		wantStatus string
	}{
		{"default column", sdk.FinishOpts{}, "done"},
		{"custom column", sdk.FinishOpts{Column: "review"}, "review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Finish me", spec, sdk.TaskAddOpts{Author: "alice"})
			moveTask(t, c, task.Key, "in-progress")
			result, err := c.Tasks.Finish(task.Key, "alice", tt.opts)
			if err != nil {
				t.Fatalf("Finish(%+v) error = %v", tt.opts, err)
			}
			if result.Task.Status != tt.wantStatus {
				t.Errorf("Finish(%+v) status = %q, want %q", tt.opts, result.Task.Status, tt.wantStatus)
			}
		})
	}
}

func TestTasksByBranch(t *testing.T) {
	c := testHost(t)

	task := addTask(t, c, "Branched", nil, sdk.TaskAddOpts{Author: "alice", Branch: "feature/login"})

	found, err := c.Tasks.ByBranch("feature/login")
	if err != nil {
		t.Fatalf("ByBranch: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("ByBranch(feature/login) key = %q, want %q", found.Key, task.Key)
	}
}

// TestTasksAdd checks the task returned by Add and the same task read
// back, plus the spec document when a body is supplied. want.Path ""
// means any non-empty generated path.
func TestTasksAdd(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  []byte
		opts  sdk.TaskAddOpts
		setup func(t *testing.T, c sdk.Context)
		want  sdk.Task
	}{
		{
			name:  "defaults",
			title: "Fix the bug",
			opts:  sdk.TaskAddOpts{Author: "alice"},
			want:  sdk.Task{Title: "Fix the bug", Status: "backlog"},
		},
		{
			name:  "with body",
			title: "Auth fix",
			body:  []byte("## Spec\n\nFix the auth tokens."),
			opts:  sdk.TaskAddOpts{Author: "alice"},
			want:  sdk.Task{Title: "Auth fix", Status: "backlog"},
		},
		{
			name:  "with column",
			title: "Urgent",
			opts:  sdk.TaskAddOpts{Author: "alice", Status: "up-next"},
			want:  sdk.Task{Title: "Urgent", Status: "up-next"},
		},
		{
			name:  "with priority",
			title: "High pri",
			opts:  sdk.TaskAddOpts{Author: "alice", Priority: 1},
			want:  sdk.Task{Title: "High pri", Status: "backlog", Priority: 1},
		},
		{
			name:  "with existing path",
			title: "Auth work",
			opts:  sdk.TaskAddOpts{Author: "alice", Path: "specs/auth"},
			setup: func(t *testing.T, c sdk.Context) { writeDoc(t, c, "specs/auth", "# Auth Spec") },
			want:  sdk.Task{Title: "Auth work", Status: "backlog", Path: "specs/auth"},
		},
		{
			name:  "with assignee",
			title: "Assigned",
			opts:  sdk.TaskAddOpts{Author: "alice", AssignedTo: "bob"},
			want:  sdk.Task{Title: "Assigned", Status: "backlog", AssignedTo: "bob"},
		},
		{
			name:  "with branch",
			title: "Branched",
			opts:  sdk.TaskAddOpts{Author: "alice", Branch: "feature-login"},
			want:  sdk.Task{Title: "Branched", Status: "backlog", Branch: "feature-login"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			task, err := c.Tasks.Add(tt.title, tt.body, tt.opts)
			if err != nil {
				t.Fatalf("Add(%q, %+v) error = %v", tt.title, tt.opts, err)
			}
			if task.Key == "" {
				t.Error("Add() key is empty")
			}
			if task.Path == "" {
				t.Error("Add() path is empty")
			}
			views := []struct {
				label string
				got   *sdk.Task
			}{{"Add()", task}, {"Read()", readTask(t, c, task.Key)}}
			for _, v := range views {
				label, got := v.label, v.got
				if got.Title != tt.want.Title {
					t.Errorf("%s title = %q, want %q", label, got.Title, tt.want.Title)
				}
				if got.Status != tt.want.Status {
					t.Errorf("%s status = %q, want %q", label, got.Status, tt.want.Status)
				}
				if got.Priority != tt.want.Priority {
					t.Errorf("%s priority = %d, want %d", label, got.Priority, tt.want.Priority)
				}
				if got.AssignedTo != tt.want.AssignedTo {
					t.Errorf("%s assigned_to = %q, want %q", label, got.AssignedTo, tt.want.AssignedTo)
				}
				if got.Branch != tt.want.Branch {
					t.Errorf("%s branch = %q, want %q", label, got.Branch, tt.want.Branch)
				}
				if tt.want.Path != "" && got.Path != tt.want.Path {
					t.Errorf("%s path = %q, want %q", label, got.Path, tt.want.Path)
				}
			}
			if tt.body != nil {
				ok, _ := c.Documents.Exists(task.Path)
				if !ok {
					t.Errorf("Exists(%q) = false, want spec document", task.Path)
				}
				content, _ := c.Documents.Read(task.Path, sdk.ReadOpts{})
				if string(content) != string(tt.body) {
					t.Errorf("Read(%q) = %q, want %q", task.Path, content, tt.body)
				}
			}
		})
	}
}

func TestTasksRead(t *testing.T) {
	c := testHost(t)

	created := addTask(t, c, "Read me", nil, sdk.TaskAddOpts{Author: "alice"})

	task, err := c.Tasks.Read(created.Key)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if task.Title != "Read me" {
		t.Errorf("Read(%q) title = %q, want %q", created.Key, task.Title, "Read me")
	}
}

func TestTasksList(t *testing.T) {
	type seed struct {
		title string
		opts  sdk.TaskAddOpts
	}
	tests := []struct {
		name string
		seed []seed
		opts sdk.TaskListOpts
		want []string // sorted titles
	}{
		{
			name: "all",
			seed: []seed{
				{"One", sdk.TaskAddOpts{Author: "alice"}},
				{"Two", sdk.TaskAddOpts{Author: "alice"}},
				{"Three", sdk.TaskAddOpts{Author: "alice", Status: "up-next"}},
			},
			opts: sdk.TaskListOpts{},
			want: []string{"One", "Three", "Two"},
		},
		{
			name: "filter by column",
			seed: []seed{
				{"Backlog", sdk.TaskAddOpts{Author: "alice"}},
				{"Next", sdk.TaskAddOpts{Author: "alice", Status: "up-next"}},
			},
			opts: sdk.TaskListOpts{Status: "up-next"},
			want: []string{"Next"},
		},
		{
			name: "filter by assignee",
			seed: []seed{
				{"Alice task", sdk.TaskAddOpts{Author: "alice", AssignedTo: "alice"}},
				{"Bob task", sdk.TaskAddOpts{Author: "bob", AssignedTo: "bob"}},
			},
			opts: sdk.TaskListOpts{AssignedTo: "alice"},
			want: []string{"Alice task"},
		},
		{
			name: "filter by priority",
			seed: []seed{
				{"Normal", sdk.TaskAddOpts{Author: "alice"}},
				{"Urgent", sdk.TaskAddOpts{Author: "alice", Priority: 1}},
				{"Critical", sdk.TaskAddOpts{Author: "alice", Priority: 2}},
			},
			opts: sdk.TaskListOpts{Priority: 1},
			want: []string{"Urgent"},
		},
		{
			name: "empty store",
			opts: sdk.TaskListOpts{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			for _, s := range tt.seed {
				addTask(t, c, s.title, nil, s.opts)
			}
			tasks, err := c.Tasks.List(tt.opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tt.opts, err)
			}
			var got []string
			for _, task := range tasks {
				got = append(got, task.Title)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%+v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
	}
}

// TestTasksMove moves a task through the listed columns in order and
// checks the error of the first failing move (if any) and the final
// status. Tasks cannot leave the backlog without a spec document.
func TestTasksMove(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		columns    []string
		wantErr    error
		wantStatus string
	}{
		{"with spec", spec, []string{"in-progress"}, nil, "in-progress"},
		{"across columns", spec, []string{"up-next", "in-progress", "review", "done"}, nil, "done"},
		{"no spec", nil, []string{"in-progress"}, sdk.ErrNoSpec, "backlog"},
		{"unknown column", spec, []string{"nope"}, sdk.ErrInvalidArg, "backlog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Movable", tt.body, sdk.TaskAddOpts{Author: "alice"})
			var err error
			var col string
			for _, col = range tt.columns {
				if err = c.Tasks.Move(task.Key, col, "alice"); err != nil {
					break
				}
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Move(%q, %q) error = %v, want %v", task.Key, col, err, tt.wantErr)
			}
			if got := readTask(t, c, task.Key).Status; got != tt.wantStatus {
				t.Errorf("Read(%q) status = %q, want %q", task.Key, got, tt.wantStatus)
			}
		})
	}
}

// TestTasksSet applies each TaskSetOpts in order and compares the
// mutable metadata read back afterwards.
func TestTasksSet(t *testing.T) {
	tests := []struct {
		name  string
		title string
		ops   []sdk.TaskSetOpts
		want  sdk.Task
	}{
		{
			name:  "title and priority",
			title: "Original",
			ops:   []sdk.TaskSetOpts{{Title: ptr("Updated"), Priority: ptr(1)}},
			want:  sdk.Task{Title: "Updated", Priority: 1},
		},
		{
			name:  "assignee",
			title: "Assign me",
			ops:   []sdk.TaskSetOpts{{AssignedTo: ptr("bob")}},
			want:  sdk.Task{Title: "Assign me", AssignedTo: "bob"},
		},
		{
			name:  "branch",
			title: "Set branch",
			ops:   []sdk.TaskSetOpts{{Branch: ptr("feature-new")}},
			want:  sdk.Task{Title: "Set branch", Branch: "feature-new"},
		},
		{
			name:  "flag",
			title: "Flagged",
			ops:   []sdk.TaskSetOpts{{Flag: "blocked"}},
			want:  sdk.Task{Title: "Flagged", Flags: "blocked"},
		},
		{
			name:  "flag then unflag",
			title: "Flagged",
			ops:   []sdk.TaskSetOpts{{Flag: "blocked"}, {Unflag: "blocked"}},
			want:  sdk.Task{Title: "Flagged", Flags: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, tt.title, nil, sdk.TaskAddOpts{Author: "alice"})
			for _, op := range tt.ops {
				if err := c.Tasks.Set(task.Key, "alice", op); err != nil {
					t.Fatalf("Set(%+v) error = %v", op, err)
				}
			}
			got := readTask(t, c, task.Key)
			if got.Title != tt.want.Title {
				t.Errorf("title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("priority = %d, want %d", got.Priority, tt.want.Priority)
			}
			if got.AssignedTo != tt.want.AssignedTo {
				t.Errorf("assigned_to = %q, want %q", got.AssignedTo, tt.want.AssignedTo)
			}
			if got.Branch != tt.want.Branch {
				t.Errorf("branch = %q, want %q", got.Branch, tt.want.Branch)
			}
			if got.Flags != tt.want.Flags {
				t.Errorf("flags = %q, want %q", got.Flags, tt.want.Flags)
			}
		})
	}
}

func TestTasksDeleteRestore(t *testing.T) {
	c := testHost(t)

	task := addTask(t, c, "Deletable", nil, sdk.TaskAddOpts{Author: "alice"})

	deleted, err := c.Tasks.Delete(task.Key, "alice")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if deleted.Title != "Deletable" {
		t.Errorf("deleted title = %q, want %q", deleted.Title, "Deletable")
	}

	// Should not appear in list
	tasks, _ := c.Tasks.List(sdk.TaskListOpts{})
	if len(tasks) != 0 {
		t.Errorf("List after delete: got %d, want 0", len(tasks))
	}

	restored, err := c.Tasks.Restore(task.Key, "alice")
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.Title != "Deletable" {
		t.Errorf("restored title = %q, want %q", restored.Title, "Deletable")
	}

	tasks, _ = c.Tasks.List(sdk.TaskListOpts{})
	if len(tasks) != 1 {
		t.Errorf("List after restore: got %d, want 1", len(tasks))
	}
}

// TestTasksColumns checks the board column order after each column
// operation.
func TestTasksColumns(t *testing.T) {
	defaults := []string{"backlog", "up-next", "in-progress", "review", "approval", "done", "blocked"}
	tests := []struct {
		name  string
		setup func(t *testing.T, c sdk.Context)
		want  []string
	}{
		{
			name: "defaults",
			want: defaults,
		},
		{
			name: "add after column",
			setup: func(t *testing.T, c sdk.Context) {
				if err := c.Tasks.AddColumn("staging", "in-progress", "alice"); err != nil {
					t.Fatalf("AddColumn(staging, in-progress) error = %v", err)
				}
			},
			want: []string{"backlog", "up-next", "in-progress", "staging", "review", "approval", "done", "blocked"},
		},
		{
			name: "add then remove",
			setup: func(t *testing.T, c sdk.Context) {
				if err := c.Tasks.AddColumn("temp", "", "alice"); err != nil {
					t.Fatalf("AddColumn(temp) error = %v", err)
				}
				if err := c.Tasks.RemoveColumn("temp", "alice"); err != nil {
					t.Fatalf("RemoveColumn(temp) error = %v", err)
				}
			},
			want: defaults,
		},
		{
			name: "add then move",
			setup: func(t *testing.T, c sdk.Context) {
				if err := c.Tasks.AddColumn("qa", "", "alice"); err != nil {
					t.Fatalf("AddColumn(qa) error = %v", err)
				}
				if err := c.Tasks.MoveColumn("qa", "review", "alice"); err != nil {
					t.Fatalf("MoveColumn(qa, review) error = %v", err)
				}
			},
			want: []string{"backlog", "up-next", "in-progress", "review", "qa", "approval", "done", "blocked"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			got, err := c.Tasks.Columns()
			if err != nil {
				t.Fatalf("Columns() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Columns() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTasksLog moves a task through the listed columns, then checks
// the event count and, when wantMoved is set, the first "moved" event.
func TestTasksLog(t *testing.T) {
	tests := []struct {
		name      string
		moves     []string
		limit     int
		wantLen   int
		wantMoved sdk.TaskEvent // zero value skips the check
	}{
		{
			name:      "records move",
			moves:     []string{"in-progress"},
			limit:     0,
			wantLen:   2, // created + moved
			wantMoved: sdk.TaskEvent{Actor: "alice", Action: "moved", OldValue: "backlog", NewValue: "in-progress"},
		},
		{
			name:    "limit",
			moves:   []string{"up-next", "in-progress", "review"},
			limit:   2,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Logged", spec, sdk.TaskAddOpts{Author: "alice"})
			for _, col := range tt.moves {
				moveTask(t, c, task.Key, col)
			}
			events, err := c.Tasks.Log(task.Key, tt.limit)
			if err != nil {
				t.Fatalf("Log(%q, %d) error = %v", task.Key, tt.limit, err)
			}
			if len(events) != tt.wantLen {
				t.Errorf("Log(%q, %d) returned %d events, want %d", task.Key, tt.limit, len(events), tt.wantLen)
			}
			if tt.wantMoved == (sdk.TaskEvent{}) {
				return
			}
			i := slices.IndexFunc(events, func(e sdk.TaskEvent) bool { return e.Action == "moved" })
			if i < 0 {
				t.Fatalf("Log(%q, %d) has no moved event: %+v", task.Key, tt.limit, events)
			}
			got := events[i]
			if got.Actor != tt.wantMoved.Actor || got.OldValue != tt.wantMoved.OldValue || got.NewValue != tt.wantMoved.NewValue {
				t.Errorf("Log(%q, %d) moved event = {Actor:%q OldValue:%q NewValue:%q}, want {Actor:%q OldValue:%q NewValue:%q}",
					task.Key, tt.limit, got.Actor, got.OldValue, got.NewValue,
					tt.wantMoved.Actor, tt.wantMoved.OldValue, tt.wantMoved.NewValue)
			}
		})
	}
}
