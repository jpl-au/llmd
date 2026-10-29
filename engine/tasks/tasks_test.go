package tasks

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"

	"github.com/jpl-au/llmd/sqlite/sqlitetest"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/engine/audit"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/events"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/task"
	intsql "github.com/jpl-au/llmd/sql"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) *Tasks {
	t.Helper()
	db := sqlitetest.Open(t, "work.db")

	if err := intsql.Exec(db); err != nil {
		t.Fatal(err)
	}

	bus := events.New()
	docs := documents.New(db, bus)
	dbFn := func() *qwr.Manager { return db }
	aud := audit.New(dbFn)

	return New(dbFn, docs, aud, bus)
}

// TestAdd covers task creation: option handling, the generated spec
// path, whether a spec document is written, and the validation errors.
// Every successful case is checked twice, on the returned task and on
// the task read back from the store.
func TestAdd(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}

	tests := []struct {
		name         string
		pre          func(t *testing.T, ts *Tasks) // state before the Add under test
		title        string
		body         []byte
		opts         AddOptions
		wantErr      error
		wantStatus   string
		wantPriority int
		wantPath     string
		wantBranch   string
		wantDoc      bool // spec document exists at wantPath holding body
	}{
		{
			name:         "defaults",
			title:        "Fix auth bug",
			body:         []byte("## Spec\n\nFix the bug."),
			opts:         AddOptions{Origin: origin, Priority: 1},
			wantStatus:   "backlog",
			wantPriority: 1,
			wantPath:     "tasks/fix-auth-bug",
			wantDoc:      true,
		},
		{
			name:       "custom status",
			title:      "Urgent fix",
			opts:       AddOptions{Origin: origin, Status: "up-next"},
			wantStatus: "up-next",
			wantPath:   "tasks/urgent-fix",
		},
		{
			name:    "invalid column",
			title:   "Bad status",
			opts:    AddOptions{Origin: origin, Status: "nonexistent"},
			wantErr: ErrInvalidCol,
		},
		{
			name:    "missing title",
			title:   "",
			opts:    AddOptions{Origin: origin},
			wantErr: ErrMissingTitle,
		},
		{
			// No body and no --path: the task has a path but no document.
			name:       "without body",
			title:      "No body task",
			opts:       AddOptions{Origin: origin},
			wantStatus: "backlog",
			wantPath:   "tasks/no-body-task",
		},
		{
			name:       "with body",
			title:      "With body",
			body:       []byte("## Spec\n\nReal content here."),
			opts:       AddOptions{Origin: origin},
			wantStatus: "backlog",
			wantPath:   "tasks/with-body",
			wantDoc:    true,
		},
		{
			// Two live tasks must never share a spec path, even when
			// neither has written a document yet.
			name: "duplicate title gets unique path",
			pre: func(t *testing.T, ts *Tasks) {
				if _, err := ts.Add(t.Context(), "Dup", nil, AddOptions{Origin: origin}); err != nil {
					t.Fatalf("Add(Dup) error = %v", err)
				}
			},
			title:      "Dup",
			opts:       AddOptions{Origin: origin},
			wantStatus: "backlog",
			wantPath:   "tasks/dup-2",
		},
		{
			name:       "with branch",
			title:      "Branched task",
			opts:       AddOptions{Origin: origin, Branch: "feature-auth"},
			wantStatus: "backlog",
			wantPath:   "tasks/branched-task",
			wantBranch: "feature-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()
			if tt.pre != nil {
				tt.pre(t, ts)
			}

			tsk, err := ts.Add(ctx, tt.title, tt.body, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%q) error = %v, want %v", tt.title, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			got, err := ts.Read(ctx, tsk.Key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", tsk.Key, err)
			}

			for _, v := range []struct {
				label string
				tsk   *task.Task
			}{{"Add", tsk}, {"Read", got}} {
				if v.tsk.Title != tt.title {
					t.Errorf("%s(%q).Title = %q, want %q", v.label, tt.title, v.tsk.Title, tt.title)
				}
				if v.tsk.Status != tt.wantStatus {
					t.Errorf("%s(%q).Status = %q, want %q", v.label, tt.title, v.tsk.Status, tt.wantStatus)
				}
				if v.tsk.Priority != tt.wantPriority {
					t.Errorf("%s(%q).Priority = %d, want %d", v.label, tt.title, v.tsk.Priority, tt.wantPriority)
				}
				if v.tsk.Path != tt.wantPath {
					t.Errorf("%s(%q).Path = %q, want %q", v.label, tt.title, v.tsk.Path, tt.wantPath)
				}
				if v.tsk.Branch != tt.wantBranch {
					t.Errorf("%s(%q).Branch = %q, want %q", v.label, tt.title, v.tsk.Branch, tt.wantBranch)
				}
			}

			doc, err := ts.docs.Read(ctx, tsk.Path)
			if (err == nil) != tt.wantDoc {
				t.Fatalf("docs.Read(%q) error = %v, want document exists %v", tsk.Path, err, tt.wantDoc)
			}
			if tt.wantDoc && doc.Content != string(tt.body) {
				t.Errorf("docs.Read(%q).Content = %q, want %q", tsk.Path, doc.Content, string(tt.body))
			}
		})
	}
}

func TestReadAndList(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
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

// TestList covers every ListOptions filter against one fixed board.
// Results are compared as sorted titles because tasks in different
// columns can share a position and a creation millisecond.
func TestList(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}

	tests := []struct {
		name string
		opts ListOptions
		want []string // sorted titles
	}{
		{"all", ListOptions{}, []string{"Alpha", "Beta", "Gamma"}},
		{"by status backlog", ListOptions{Status: "backlog"}, []string{"Alpha", "Gamma"}},
		{"by status up-next", ListOptions{Status: "up-next"}, []string{"Beta"}},
		{"by assignee", ListOptions{AssignedTo: "alice"}, []string{"Alpha"}},
		{"by priority", ListOptions{Priority: 1}, []string{"Alpha", "Gamma"}},
		{"by branch", ListOptions{Branch: "feat-a"}, []string{"Alpha"}},
		{"combined", ListOptions{Status: "backlog", Priority: 1, AssignedTo: "alice"}, []string{"Alpha"}},
		{"no match", ListOptions{AssignedTo: "carol"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()

			for _, a := range []struct {
				title string
				opts  AddOptions
			}{
				{"Alpha", AddOptions{Origin: origin, AssignedTo: "alice", Priority: 1, Branch: "feat-a"}},
				{"Beta", AddOptions{Origin: origin, Status: "up-next", AssignedTo: "bob", Priority: 2, Branch: "feat-b"}},
				{"Gamma", AddOptions{Origin: origin, Priority: 1}},
			} {
				if _, err := ts.Add(ctx, a.title, nil, a.opts); err != nil {
					t.Fatalf("Add(%q) error = %v", a.title, err)
				}
			}

			tasks, err := ts.List(ctx, tt.opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tt.opts, err)
			}
			var got []string
			for _, tsk := range tasks {
				got = append(got, tsk.Title)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%+v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
	}
}

// TestMove covers column changes from the backlog, including spec
// gating and column validation. A rejected move must leave the task
// where it was.
func TestMove(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}

	tests := []struct {
		name    string
		body    []byte
		status  string
		wantErr error
	}{
		{"with spec", []byte("# Specced task\n\nReal content here."), "up-next", nil},
		{"no spec", nil, "up-next", ErrNoSpec},
		{"heading only spec", []byte("# Specced task"), "up-next", ErrNoSpec},
		{"backlog to backlog needs no spec", nil, "backlog", nil},
		{"invalid column", []byte("# Specced task\n\nReal content here."), "nonexistent", ErrInvalidCol},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()

			tsk, err := ts.Add(ctx, "Specced task", tt.body, AddOptions{Origin: origin})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}

			err = ts.Move(ctx, tsk.Key, tt.status, "alice")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Move(%q, %q) error = %v, want %v", tsk.Key, tt.status, err, tt.wantErr)
			}

			wantStatus := "backlog"
			if tt.wantErr == nil {
				wantStatus = tt.status
			}
			got, err := ts.Read(ctx, tsk.Key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", tsk.Key, err)
			}
			if got.Status != wantStatus {
				t.Errorf("after Move(%q, %q): Status = %q, want %q", tsk.Key, tt.status, got.Status, wantStatus)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
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
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read(%q) error = %v, want ErrNotFound", tsk.Key, err)
	}
}

func TestRestore(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "To restore", nil, AddOptions{Origin: origin})
	if _, err := ts.Delete(ctx, tsk.Key, "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

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

// TestSet covers each metadata field Set can change. Cases that depend
// on earlier state (a second flag, clearing a value) build it through
// add options or a setup step, then one Set call is checked against
// the task read back from the store.
func TestSet(t *testing.T) {
	const title = "Set target"
	origin := core.Origin{Author: "alice", Source: "test"}

	tests := []struct {
		name  string
		add   AddOptions                                // options for the task under test
		setup func(t *testing.T, ts *Tasks, key string) // prior Set calls, if any
		opts  SetOptions
		want  task.Task // Title, Priority, AssignedTo, Branch and Flags are compared
	}{
		{
			name: "add flag",
			opts: SetOptions{Flag: "blocked"},
			want: task.Task{Title: title, Flags: "blocked"},
		},
		{
			name: "add second flag",
			setup: func(t *testing.T, ts *Tasks, key string) {
				if err := ts.Set(t.Context(), key, "alice", SetOptions{Flag: "blocked"}); err != nil {
					t.Fatalf("Set flag blocked: %v", err)
				}
			},
			opts: SetOptions{Flag: "hold"},
			want: task.Task{Title: title, Flags: "blocked,hold"},
		},
		{
			name: "remove flag",
			setup: func(t *testing.T, ts *Tasks, key string) {
				for _, f := range []string{"blocked", "hold"} {
					if err := ts.Set(t.Context(), key, "alice", SetOptions{Flag: f}); err != nil {
						t.Fatalf("Set flag %s: %v", f, err)
					}
				}
			},
			opts: SetOptions{Unflag: "blocked"},
			want: task.Task{Title: title, Flags: "hold"},
		},
		{
			name: "set branch",
			opts: SetOptions{Branch: new("feature-xyz")},
			want: task.Task{Title: title, Branch: "feature-xyz"},
		},
		{
			name: "clear branch",
			add:  AddOptions{Branch: "old-branch"},
			opts: SetOptions{Branch: new("")},
			want: task.Task{Title: title},
		},
		{
			name: "set title",
			opts: SetOptions{Title: new("Renamed")},
			want: task.Task{Title: "Renamed"},
		},
		{
			name: "set priority",
			opts: SetOptions{Priority: new(3)},
			want: task.Task{Title: title, Priority: 3},
		},
		{
			name: "set assignee",
			opts: SetOptions{AssignedTo: new("bob")},
			want: task.Task{Title: title, AssignedTo: "bob"},
		},
		{
			name: "clear assignee",
			add:  AddOptions{AssignedTo: "bob"},
			opts: SetOptions{AssignedTo: new("")},
			want: task.Task{Title: title},
		},
		{
			name: "several fields at once",
			opts: SetOptions{Title: new("Renamed"), Priority: new(2), Flag: "blocked"},
			want: task.Task{Title: "Renamed", Priority: 2, Flags: "blocked"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()

			add := tt.add
			add.Origin = origin
			tsk, err := ts.Add(ctx, title, nil, add)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, ts, tsk.Key)
			}

			if err := ts.Set(ctx, tsk.Key, "alice", tt.opts); err != nil {
				t.Fatalf("Set(%q) error = %v", tsk.Key, err)
			}

			got, err := ts.Read(ctx, tsk.Key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", tsk.Key, err)
			}
			if got.Title != tt.want.Title {
				t.Errorf("after Set: Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("after Set: Priority = %d, want %d", got.Priority, tt.want.Priority)
			}
			if got.AssignedTo != tt.want.AssignedTo {
				t.Errorf("after Set: AssignedTo = %q, want %q", got.AssignedTo, tt.want.AssignedTo)
			}
			if got.Branch != tt.want.Branch {
				t.Errorf("after Set: Branch = %q, want %q", got.Branch, tt.want.Branch)
			}
			if got.Flags != tt.want.Flags {
				t.Errorf("after Set: Flags = %q, want %q", got.Flags, tt.want.Flags)
			}
		})
	}
}

// TestPosition covers repositioning within a column. Three tasks are
// created in order; one is moved to a position and the resulting
// order is checked, with out-of-range positions clamped.
func TestPosition(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}
	titles := []string{"First", "Second", "Third"}

	tests := []struct {
		name string
		move int // index of the task to move
		pos  int
		want []int // expected order as indices
	}{
		{"third to top", 2, 0, []int{2, 0, 1}},
		{"first to bottom", 0, 2, []int{1, 2, 0}},
		{"first to middle", 0, 1, []int{1, 0, 2}},
		{"position beyond end clamps to bottom", 0, 99, []int{1, 2, 0}},
		{"negative position clamps to top", 2, -1, []int{2, 0, 1}},
		{"no-op", 1, 1, []int{0, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()

			var keys []string
			for _, title := range titles {
				tsk, err := ts.Add(ctx, title, nil, AddOptions{Origin: origin})
				if err != nil {
					t.Fatalf("Add(%q) error = %v", title, err)
				}
				keys = append(keys, tsk.Key)
			}

			if err := ts.Set(ctx, keys[tt.move], "alice", SetOptions{Position: new(tt.pos)}); err != nil {
				t.Fatalf("Set(Position=%d) error = %v", tt.pos, err)
			}

			all, err := ts.List(ctx, ListOptions{})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			var got []string
			for _, tsk := range all {
				got = append(got, tsk.Key)
			}
			var want []string
			for _, i := range tt.want {
				want = append(want, keys[i])
			}
			if !slices.Equal(got, want) {
				t.Errorf("order after Set(%s, Position=%d) = %v, want %v", titles[tt.move], tt.pos, got, want)
			}
		})
	}
}

func TestColumns(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
	origin := core.Origin{Author: "alice", Source: "test"}

	// Trigger board creation
	if _, err := ts.Add(ctx, "Trigger", nil, AddOptions{Origin: origin}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	cols, err := ts.Columns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 7 {
		t.Fatalf("len = %d, want 7", len(cols))
	}

	// Add column
	if err := ts.AddColumn(ctx, "staging", "review", "alice"); err != nil {
		t.Fatal(err)
	}
	cols, err = ts.Columns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 8 {
		t.Fatalf("len = %d, want 8", len(cols))
	}

	// Remove column
	if err := ts.RemoveColumn(ctx, "staging", "alice"); err != nil {
		t.Fatal(err)
	}
	cols, err = ts.Columns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 7 {
		t.Fatalf("len = %d, want 7", len(cols))
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "Fix auth bug", "fix-auth-bug"},
		{"uppercase", "Update API docs", "update-api-docs"},
		{"punctuation", "Hello, World!", "hello-world"},
		{"surrounding and repeated spaces", "  spaces  everywhere  ", "spaces-everywhere"},
		{"already slugged", "already-slugged", "already-slugged"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slug(tt.input)
			if got != tt.want {
				t.Errorf("slug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogSingleTask(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
	origin := core.Origin{Author: "alice", Source: "test"}

	tsk, _ := ts.Add(ctx, "Logged task", []byte("# Logged task\n\nSpec content."), AddOptions{Origin: origin})

	// Move and set to generate audit entries
	if err := ts.Move(ctx, tsk.Key, "up-next", "alice"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if err := ts.Set(ctx, tsk.Key, "bob", SetOptions{Flag: "blocked"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

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
	ctx := t.Context()
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

// TestLogLimit covers the limit argument against a task with three
// audit events (created, moved, flagged).
func TestLogLimit(t *testing.T) {
	origin := core.Origin{Author: "alice", Source: "test"}

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"no limit", 0, 3},
		{"limit one", 1, 1},
		{"limit two", 2, 2},
		{"limit beyond count", 10, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()

			tsk, err := ts.Add(ctx, "Many events", []byte("# Many events\n\nSpec."), AddOptions{Origin: origin})
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if err := ts.Move(ctx, tsk.Key, "up-next", "alice"); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if err := ts.Set(ctx, tsk.Key, "alice", SetOptions{Flag: "hold"}); err != nil {
				t.Fatalf("Set: %v", err)
			}

			events, err := ts.Log(ctx, tsk.Key, tt.limit)
			if err != nil {
				t.Fatalf("Log(%q, %d) error = %v", tsk.Key, tt.limit, err)
			}
			if len(events) != tt.want {
				t.Errorf("len(Log(%q, %d)) = %d, want %d", tsk.Key, tt.limit, len(events), tt.want)
			}
		})
	}
}

func TestLogNotFound(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()

	_, err := ts.Log(ctx, "nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestListSince(t *testing.T) {
	clocktest.Sequence(t, 1_000, 1)
	ts := setup(t)
	ctx := t.Context()
	origin := core.Origin{Author: "alice", Source: "test"}

	if _, err := ts.Add(ctx, "Old task", nil, AddOptions{Origin: origin}); err != nil {
		t.Fatalf("Add(Old task) error = %v", err)
	}

	// Record a timestamp after the first task.
	all, _ := ts.List(ctx, ListOptions{})
	if len(all) != 1 {
		t.Fatalf("expected 1 task, got %d", len(all))
	}
	cutoff := all[0].CreatedAt

	if _, err := ts.Add(ctx, "New task", nil, AddOptions{Origin: origin}); err != nil {
		t.Fatalf("Add(New task) error = %v", err)
	}

	// Since = cutoff should return only the second task.
	recent, err := ts.List(ctx, ListOptions{SinceMS: cutoff})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent task, got %d", len(recent))
	}
	if recent[0].Title != "New task" {
		t.Errorf("title = %q, want %q", recent[0].Title, "New task")
	}
}
