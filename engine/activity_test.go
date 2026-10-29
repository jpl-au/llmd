package engine_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"
	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/tasks"
)

// taskKey is the Subject placeholder for task events in activity
// tables: task keys are generated, so setup returns the real key and
// the test substitutes it before comparing.
const taskKey = "<task>"

func testTaskOpts() tasks.AddOptions {
	return tasks.AddOptions{Author: "test", Source: "cli"}
}

// addTask creates a task with a real spec so it can leave the backlog.
func addTask(t *testing.T, s *engine.Store, title string) string {
	t.Helper()
	tsk, err := s.Tasks.Add(t.Context(), title, []byte("# "+title+"\n\nBody."), testTaskOpts())
	if err != nil {
		t.Fatalf("Tasks.Add(%q) error = %v", title, err)
	}
	return tsk.Key
}

// insertEntity writes a raw tag or link row, bypassing the domain
// packages, so the feed can be fed values they would never store.
func insertEntity(t *testing.T, s *engine.Store, namespace, relation, value string) {
	t.Helper()
	_, err := s.DB().Query(`
		INSERT INTO entities (key, namespace, relation, value, author, source, created_at)
		VALUES ('raw000001', ?, ?, ?, 'test', 'cli', 1000)
	`, namespace, relation, value).WithContext(t.Context()).Exec()
	if err != nil {
		t.Fatalf("inserting %s entity: %v", namespace, err)
	}
}

// TestRecentActivity_Shape checks the entry each source produces: one
// case per type and action, at a fixed clock so timestamps are exact.
// Task cases use limit 1 because adding a task with a spec also writes
// its document; the newest entry is the one under test.
func TestRecentActivity_Shape(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *engine.Store) string // returns the task key, if any
		limit int
		want  []engine.ActivityEvent
	}{
		{
			name: "document written",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				opts := testWriteOpts()
				opts.Message = "first draft"
				if _, err := s.Documents.Write(t.Context(), "docs/a", "hello", opts); err != nil {
					t.Fatalf("Write(docs/a) error = %v", err)
				}
				return ""
			},
			limit: 10,
			want: []engine.ActivityEvent{
				{Type: "document", Action: "written", Subject: "docs/a", Author: "test", Detail: "first draft", Timestamp: 1_000},
			},
		},
		{
			name: "document deleted",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				if _, err := s.Documents.Write(t.Context(), "docs/a", "hello", testWriteOpts()); err != nil {
					t.Fatalf("Write(docs/a) error = %v", err)
				}
				clocktest.Fixed(t, 2_000)
				if err := s.Documents.Delete(t.Context(), "docs/a", testDeleteOpts()); err != nil {
					t.Fatalf("Delete(docs/a) error = %v", err)
				}
				return ""
			},
			limit: 10,
			want: []engine.ActivityEvent{
				{Type: "document", Action: "deleted", Subject: "docs/a", Author: "test", Timestamp: 2_000},
			},
		},
		{
			name: "tagged",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				if _, err := s.Documents.Write(t.Context(), "docs/a", "hello", testWriteOpts()); err != nil {
					t.Fatalf("Write(docs/a) error = %v", err)
				}
				clocktest.Fixed(t, 2_000)
				if _, err := s.Tags.Add(t.Context(), "docs/a", "urgent", testTagOpts()); err != nil {
					t.Fatalf("Tags.Add() error = %v", err)
				}
				return ""
			},
			limit: 1,
			want: []engine.ActivityEvent{
				{Type: "tag", Action: "tagged", Subject: "docs/a", Author: "test", Detail: "urgent", Timestamp: 2_000},
			},
		},
		{
			name: "untagged",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				if _, err := s.Documents.Write(t.Context(), "docs/a", "hello", testWriteOpts()); err != nil {
					t.Fatalf("Write(docs/a) error = %v", err)
				}
				if _, err := s.Tags.Add(t.Context(), "docs/a", "urgent", testTagOpts()); err != nil {
					t.Fatalf("Tags.Add() error = %v", err)
				}
				clocktest.Fixed(t, 2_000)
				if err := s.Tags.Remove(t.Context(), "docs/a", "urgent", testTagOpts()); err != nil {
					t.Fatalf("Tags.Remove() error = %v", err)
				}
				return ""
			},
			limit: 1,
			want: []engine.ActivityEvent{
				{Type: "tag", Action: "untagged", Subject: "docs/a", Author: "test", Detail: "urgent", Timestamp: 2_000},
			},
		},
		{
			name: "linked",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				for _, p := range []string{"docs/a", "docs/b"} {
					if _, err := s.Documents.Write(t.Context(), p, "hello", testWriteOpts()); err != nil {
						t.Fatalf("Write(%s) error = %v", p, err)
					}
				}
				clocktest.Fixed(t, 2_000)
				if _, err := s.Links.Add(t.Context(), "docs/a", "docs/b", testLinkOpts()); err != nil {
					t.Fatalf("Links.Add() error = %v", err)
				}
				return ""
			},
			limit: 1,
			want: []engine.ActivityEvent{
				{Type: "link", Action: "linked", Subject: "docs/a", Author: "test", Detail: "docs/b", Timestamp: 2_000},
			},
		},
		{
			name: "unlinked",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				for _, p := range []string{"docs/a", "docs/b"} {
					if _, err := s.Documents.Write(t.Context(), p, "hello", testWriteOpts()); err != nil {
						t.Fatalf("Write(%s) error = %v", p, err)
					}
				}
				if _, err := s.Links.Add(t.Context(), "docs/a", "docs/b", testLinkOpts()); err != nil {
					t.Fatalf("Links.Add() error = %v", err)
				}
				clocktest.Fixed(t, 2_000)
				if err := s.Links.Remove(t.Context(), "docs/a", "docs/b", testLinkOpts()); err != nil {
					t.Fatalf("Links.Remove() error = %v", err)
				}
				return ""
			},
			limit: 1,
			want: []engine.ActivityEvent{
				{Type: "link", Action: "unlinked", Subject: "docs/a", Author: "test", Detail: "docs/b", Timestamp: 2_000},
			},
		},
		{
			// A task without a body writes no document, so the feed
			// holds only the audit entry.
			name: "task created",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				return tsk.Key
			},
			limit: 10,
			want: []engine.ActivityEvent{
				{Type: "task", Action: "created", Subject: taskKey, Author: "test", Detail: "Fix login", Timestamp: 1_000},
			},
		},
		{
			// Both old and new values are set, so Detail joins them.
			name: "task moved",
			setup: func(t *testing.T, s *engine.Store) string {
				clocktest.Fixed(t, 1_000)
				key := addTask(t, s, "Fix login")
				clocktest.Fixed(t, 2_000)
				if err := s.Tasks.Move(t.Context(), key, "up-next", "test"); err != nil {
					t.Fatalf("Tasks.Move() error = %v", err)
				}
				return key
			},
			limit: 1,
			want: []engine.ActivityEvent{
				{Type: "task", Action: "moved", Subject: taskKey, Author: "test", Detail: "backlog → up-next", Timestamp: 2_000},
			},
		},
		{
			// A value that is not JSON is shown as stored.
			name: "tag value not json",
			setup: func(t *testing.T, s *engine.Store) string {
				insertEntity(t, s, "core:tag", "docs/a", "not-json")
				return ""
			},
			limit: 10,
			want: []engine.ActivityEvent{
				{Type: "tag", Action: "tagged", Subject: "docs/a", Author: "test", Detail: "not-json", Timestamp: 1_000},
			},
		},
		{
			// JSON without the expected field is also shown as stored.
			name: "link value missing field",
			setup: func(t *testing.T, s *engine.Store) string {
				insertEntity(t, s, "core:link", "docs/a", `{"from":"docs/a"}`)
				return ""
			},
			limit: 10,
			want: []engine.ActivityEvent{
				{Type: "link", Action: "linked", Subject: "docs/a", Author: "test", Detail: `{"from":"docs/a"}`, Timestamp: 1_000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			key := tt.setup(t, s)

			got, err := s.RecentActivity(t.Context(), tt.limit)
			if err != nil {
				t.Fatalf("RecentActivity(%d) error = %v", tt.limit, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("RecentActivity(%d) = %+v, want %+v", tt.limit, got, tt.want)
			}
			for i, want := range tt.want {
				if want.Subject == taskKey {
					want.Subject = key
				}
				if got[i] != want {
					t.Errorf("RecentActivity(%d)[%d] = %+v, want %+v", tt.limit, i, got[i], want)
				}
			}
		})
	}
}

// activityStep identifies an entry in the merged feed without its
// timestamp or author.
type activityStep struct {
	Type, Action, Subject string
}

// TestRecentActivity_Merge runs each setup under a sequence clock, so
// every write gets a later timestamp than the one before, and checks
// the feed lists entries from all sources newest first and honours the
// limit. Task subjects are generated, so a task step uses taskKey.
func TestRecentActivity_Merge(t *testing.T) {
	writeDoc := func(t *testing.T, s *engine.Store, path string) {
		t.Helper()
		if _, err := s.Documents.Write(t.Context(), path, "content "+path, testWriteOpts()); err != nil {
			t.Fatalf("Write(%s) error = %v", path, err)
		}
	}
	addTag := func(t *testing.T, s *engine.Store, path, name string) {
		t.Helper()
		if _, err := s.Tags.Add(t.Context(), path, name, testTagOpts()); err != nil {
			t.Fatalf("Tags.Add(%s, %s) error = %v", path, name, err)
		}
	}

	tests := []struct {
		name   string
		setup  func(t *testing.T, s *engine.Store) string // returns the task key, if any
		cancel bool                                       // query with an already-cancelled context
		limit  int
		want   []activityStep
	}{
		{
			name:  "empty store",
			limit: 10,
		},
		{
			name: "documents only",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/a")
				writeDoc(t, s, "docs/b")
				return ""
			},
			limit: 10,
			want: []activityStep{
				{"document", "written", "docs/b"},
				{"document", "written", "docs/a"},
			},
		},
		{
			name: "entities only",
			setup: func(t *testing.T, s *engine.Store) string {
				insertEntity(t, s, "core:tag", "docs/a", `{"tag":"urgent"}`)
				return ""
			},
			limit: 10,
			want: []activityStep{
				{"tag", "tagged", "docs/a"},
			},
		},
		{
			name: "tasks only",
			setup: func(t *testing.T, s *engine.Store) string {
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				return tsk.Key
			},
			limit: 10,
			want: []activityStep{
				{"task", "created", taskKey},
			},
		},
		{
			name: "all sources merged newest first",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/a")
				addTag(t, s, "docs/a", "urgent")
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				writeDoc(t, s, "docs/b")
				if _, err := s.Links.Add(t.Context(), "docs/a", "docs/b", testLinkOpts()); err != nil {
					t.Fatalf("Links.Add() error = %v", err)
				}
				return tsk.Key
			},
			limit: 10,
			want: []activityStep{
				{"link", "linked", "docs/a"},
				{"document", "written", "docs/b"},
				{"task", "created", taskKey},
				{"tag", "tagged", "docs/a"},
				{"document", "written", "docs/a"},
			},
		},
		{
			name: "zero limit returns everything",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/a")
				addTag(t, s, "docs/a", "urgent")
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				return tsk.Key
			},
			limit: 0,
			want: []activityStep{
				{"task", "created", taskKey},
				{"tag", "tagged", "docs/a"},
				{"document", "written", "docs/a"},
			},
		},
		{
			name: "recent delete of an old document is newest",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/old")
				writeDoc(t, s, "docs/a")
				writeDoc(t, s, "docs/b")
				if err := s.Documents.Delete(t.Context(), "docs/old", testDeleteOpts()); err != nil {
					t.Fatalf("Documents.Delete() error = %v", err)
				}
				return ""
			},
			limit: 1,
			want:  []activityStep{{"document", "deleted", "docs/old"}},
		},
		{
			name: "limit keeps the newest across sources",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/a")
				addTag(t, s, "docs/a", "urgent")
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				writeDoc(t, s, "docs/b")
				return tsk.Key
			},
			limit: 2,
			want: []activityStep{
				{"document", "written", "docs/b"},
				{"task", "created", taskKey},
			},
		},
		{
			// Every source swallows its query error, so a dead context
			// yields an empty feed rather than an error.
			name: "cancelled context",
			setup: func(t *testing.T, s *engine.Store) string {
				writeDoc(t, s, "docs/a")
				tsk, err := s.Tasks.Add(t.Context(), "Fix login", nil, testTaskOpts())
				if err != nil {
					t.Fatalf("Tasks.Add() error = %v", err)
				}
				return tsk.Key
			},
			cancel: true,
			limit:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			clocktest.Sequence(t, 1_000, 1)
			var key string
			if tt.setup != nil {
				key = tt.setup(t, s)
			}

			ctx := t.Context()
			if tt.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			got, err := s.RecentActivity(ctx, tt.limit)
			if err != nil {
				t.Fatalf("RecentActivity(%d) error = %v", tt.limit, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("RecentActivity(%d) = %+v, want %d entries %v", tt.limit, got, len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if want.Subject == taskKey {
					want.Subject = key
				}
				step := activityStep{got[i].Type, got[i].Action, got[i].Subject}
				if step != want {
					t.Errorf("RecentActivity(%d)[%d] = %+v, want %+v", tt.limit, i, step, want)
				}
				if got[i].Author != "test" {
					t.Errorf("RecentActivity(%d)[%d].Author = %q, want %q", tt.limit, i, got[i].Author, "test")
				}
				if i > 0 && got[i].Timestamp >= got[i-1].Timestamp {
					t.Errorf("RecentActivity(%d)[%d].Timestamp = %d, want < %d (newest first)", tt.limit, i, got[i].Timestamp, got[i-1].Timestamp)
				}
			}
		})
	}
}
