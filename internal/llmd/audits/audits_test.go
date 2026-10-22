package audits

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/profile"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *qwr.Manager {
	t.Helper()
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	db, err := qwr.New("file::memory:?cache=shared").Reader(rp).Writer(wp).Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})
	return db
}

// newTestStore opens a fresh in-memory database and returns an Audits
// store backed by it.
func newTestStore(t *testing.T) *Audits {
	t.Helper()
	db := openTestDB(t)
	return New(func() *qwr.Manager { return db }, nil)
}

// TestAdd covers top-level audit creation: inferred target type,
// default status, assignee, and the required-field errors. Successful
// cases are checked on the returned audit and on the stored row.
func TestAdd(t *testing.T) {
	tests := []struct {
		name    string
		opts    AddOptions
		wantErr error
		want    Audit // TargetType, Status, Assignee and ParentID are compared
	}{
		{
			name: "document target",
			opts: AddOptions{Target: "docs/api", Content: "Needs error handling.", Author: "gemini", Assignee: "claude-code"},
			want: Audit{TargetType: "document", Status: "pending", Assignee: "claude-code"},
		},
		{
			name: "task target",
			opts: AddOptions{Target: "0mmofq4de", Content: "Check the spec.", Author: "gemini"},
			want: Audit{TargetType: "task", Status: "pending"},
		},
		{
			name: "explicit status",
			opts: AddOptions{Target: "docs/api", Content: "Needs work.", Author: "gemini", Status: "needs-work"},
			want: Audit{TargetType: "document", Status: "needs-work"},
		},
		{
			name:    "missing author",
			opts:    AddOptions{Target: "docs/api", Content: "test"},
			wantErr: ErrMissingAuthor,
		},
		{
			name:    "missing target",
			opts:    AddOptions{Author: "gemini", Content: "test"},
			wantErr: ErrMissingTarget,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			aud, err := store.Add(ctx, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if aud.ID == "" {
				t.Error("Add() returned empty ID")
			}

			stored, err := store.Read(ctx, aud.ID)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", aud.ID, err)
			}

			for _, v := range []struct {
				label string
				aud   *Audit
			}{{"Add", aud}, {"Read", stored}} {
				if v.aud.TargetType != tt.want.TargetType {
					t.Errorf("%s().TargetType = %q, want %q", v.label, v.aud.TargetType, tt.want.TargetType)
				}
				if v.aud.Status != tt.want.Status {
					t.Errorf("%s().Status = %q, want %q", v.label, v.aud.Status, tt.want.Status)
				}
				if v.aud.Assignee != tt.want.Assignee {
					t.Errorf("%s().Assignee = %q, want %q", v.label, v.aud.Assignee, tt.want.Assignee)
				}
				if v.aud.ParentID != tt.want.ParentID {
					t.Errorf("%s().ParentID = %q, want %q", v.label, v.aud.ParentID, tt.want.ParentID)
				}
			}
		})
	}
}

func TestInferTargetType(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{"document path", "docs/api", "document"},
		{"nested document path", "notes/meeting", "document"},
		{"valid base36 key", "0mmofq4de", "task"},
		{"all zeros key", "000000000", "task"},
		{"short word", "docs", "document"}, // 4 chars, not a key
		{"lowercase alpha key", "abcdefghi", "task"},
		{"uppercase", "ABCDEFGHI", "document"}, // uppercase = not valid base36
		{"too short", "abc", "document"},
		{"too long", "abcdefghij", "document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTargetType(tt.target)
			if got != tt.want {
				t.Errorf("inferTargetType(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

// TestReply covers replies to a thread: inheritance of status and
// assignee from the parent, reassignment, flattening of nested replies
// to the top-level audit, and the required-author error.
func TestReply(t *testing.T) {
	tests := []struct {
		name         string
		parent       AddOptions
		nested       bool // reply to a reply instead of the top-level audit
		opts         AddOptions
		wantErr      error
		wantStatus   string
		wantAssignee string
	}{
		{
			name:         "inherits assignee",
			parent:       AddOptions{Target: "docs/api", Content: "Needs error handling.", Author: "gemini", Assignee: "claude-code"},
			opts:         AddOptions{Content: "Fixed.", Author: "claude-code", Status: "approved"},
			wantStatus:   "approved",
			wantAssignee: "claude-code",
		},
		{
			name:         "inherits status",
			parent:       AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini", Assignee: "claude-code", Status: "needs-work"},
			opts:         AddOptions{Content: "Noted.", Author: "claude-code"},
			wantStatus:   "needs-work",
			wantAssignee: "claude-code",
		},
		{
			name:         "reassigns",
			parent:       AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini", Assignee: "claude-code"},
			opts:         AddOptions{Content: "Done, please check.", Author: "claude-code", Assignee: "gemini"},
			wantStatus:   "pending",
			wantAssignee: "gemini",
		},
		{
			name:       "reply to reply flattens to top level",
			parent:     AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini"},
			nested:     true,
			opts:       AddOptions{Content: "Second reply.", Author: "gemini"},
			wantStatus: "pending",
		},
		{
			name:    "missing author",
			parent:  AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini"},
			opts:    AddOptions{Content: "Anonymous."},
			wantErr: ErrMissingAuthor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			top, err := store.Add(ctx, tt.parent)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			target := top.ID
			if tt.nested {
				first, err := store.Reply(ctx, top.ID, AddOptions{Content: "First reply.", Author: "claude-code"})
				if err != nil {
					t.Fatalf("Reply(first) error = %v", err)
				}
				target = first.ID
			}

			reply, err := store.Reply(ctx, target, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Reply(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if reply.ParentID != top.ID {
				t.Errorf("Reply().ParentID = %q, want %q (top-level)", reply.ParentID, top.ID)
			}
			if reply.Target != top.Target {
				t.Errorf("Reply().Target = %q, want %q", reply.Target, top.Target)
			}
			if reply.Status != tt.wantStatus {
				t.Errorf("Reply().Status = %q, want %q", reply.Status, tt.wantStatus)
			}
			if reply.Assignee != tt.wantAssignee {
				t.Errorf("Reply().Assignee = %q, want %q", reply.Assignee, tt.wantAssignee)
			}
		})
	}
}

// TestRead covers lookup by ID. An empty id reads the audit created
// in the subtest; anything else is used literally.
func TestRead(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantErr     error
		wantContent string
	}{
		{"existing", "", nil, "Test."},
		{"not found", "xxxxxxxxx", ErrNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			created, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "Test.", Author: "gemini"})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			id := tt.id
			if id == "" {
				id = created.ID
			}

			read, err := store.Read(ctx, id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Read(%q) error = %v, want %v", id, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if read.ID != id {
				t.Errorf("Read(%q).ID = %q, want %q", id, read.ID, id)
			}
			if read.Content != tt.wantContent {
				t.Errorf("Read(%q).Content = %q, want %q", id, read.Content, tt.wantContent)
			}
		})
	}
}

func TestThread(t *testing.T) {
	store := newTestStore(t)
	ctx := t.Context()

	top, err := store.Add(ctx, AddOptions{
		Target: "docs/api", Content: "Review.", Author: "gemini",
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := store.Reply(ctx, top.ID, AddOptions{
		Content: "Fixed.", Author: "claude-code", Status: "approved",
	}); err != nil {
		t.Fatalf("Reply() error = %v", err)
	}

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

// TestList covers every ListOptions filter against one fixed set of
// threads:
//
//	A. docs/api    by gemini      -> claude-code, pending, then resolved (approved)
//	B. docs/auth   by claude-code -> gemini,      approved
//	C. docs/api    by gemini      -> claude-code, needs-work
//
// Results are compared as sorted content because audits created in
// the same millisecond have no defined order.
func TestList(t *testing.T) {
	tests := []struct {
		name string
		opts ListOptions
		want []string // sorted content
	}{
		{"all", ListOptions{}, []string{"A.", "B.", "C."}},
		{"by target", ListOptions{Target: "docs/api"}, []string{"A.", "C."}},
		{"by author", ListOptions{ByAuthor: "gemini"}, []string{"A.", "C."}},
		{"by assignee", ListOptions{Assignee: "claude-code"}, []string{"A.", "C."}},
		// Only C remains pending: A was resolved and B was approved.
		{"pending", ListOptions{Pending: true}, []string{"C."}},
		{"by effective status approved", ListOptions{Status: "approved"}, []string{"A.", "B."}},
		{"by effective status needs-work", ListOptions{Status: "needs-work"}, []string{"C."}},
		{"pending and target", ListOptions{Pending: true, Target: "docs/auth"}, nil},
		{"no match", ListOptions{Target: "docs/none"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			a, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "A.", Author: "gemini", Assignee: "claude-code"})
			if err != nil {
				t.Fatalf("Add(A) error = %v", err)
			}
			if _, err := store.Add(ctx, AddOptions{Target: "docs/auth", Content: "B.", Author: "claude-code", Assignee: "gemini", Status: "approved"}); err != nil {
				t.Fatalf("Add(B) error = %v", err)
			}
			if _, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "C.", Author: "gemini", Assignee: "claude-code", Status: "needs-work"}); err != nil {
				t.Fatalf("Add(C) error = %v", err)
			}
			// Resolve A - its effective status becomes "approved".
			if _, err := store.Resolve(ctx, a.ID, "claude-code"); err != nil {
				t.Fatalf("Resolve(A) error = %v", err)
			}

			audits, err := store.List(ctx, tt.opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", tt.opts, err)
			}
			var got []string
			for _, aud := range audits {
				got = append(got, aud.Content)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%+v) = %v, want %v", tt.opts, got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	store := newTestStore(t)
	ctx := t.Context()

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

// TestDelete covers soft deletion. An empty id deletes the audit
// created in the subtest; anything else is used literally. A rejected
// delete must leave the audit readable.
func TestDelete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		author  string
		wantErr error
	}{
		{"deletes", "", "gemini", nil},
		{"missing author", "", "", ErrMissingAuthor},
		{"not found", "xxxxxxxxx", "gemini", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			aud, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini"})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			id := tt.id
			if id == "" {
				id = aud.ID
			}

			err = store.Delete(ctx, id, tt.author)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Delete(%q, %q) error = %v, want %v", id, tt.author, err, tt.wantErr)
			}

			// The created audit is gone only when the delete succeeded.
			var wantRead error
			if tt.wantErr == nil {
				wantRead = ErrNotFound
			}
			if _, err := store.Read(ctx, aud.ID); !errors.Is(err, wantRead) {
				t.Errorf("after Delete: Read(%q) error = %v, want %v", aud.ID, err, wantRead)
			}
		})
	}
}

// TestRestore covers undeleting a soft-deleted audit and the
// required-author error. A rejected restore must leave the audit
// deleted.
func TestRestore(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		live    bool // skip the delete so the audit is not restorable
		wantErr error
	}{
		{"restores", "gemini", false, nil},
		{"missing author", "", false, ErrMissingAuthor},
		{"rejects live audit", "gemini", true, ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			ctx := t.Context()

			aud, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini"})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if !tt.live {
				if err := store.Delete(ctx, aud.ID, "gemini"); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				if _, err := store.Read(ctx, aud.ID); !errors.Is(err, ErrNotFound) {
					t.Fatalf("after Delete: Read(%q) error = %v, want ErrNotFound", aud.ID, err)
				}
			}

			restored, err := store.Restore(ctx, aud.ID, tt.author)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Restore(%q, %q) error = %v, want %v", aud.ID, tt.author, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				_, err := store.Read(ctx, aud.ID)
				if tt.live && err != nil {
					t.Errorf("after rejected Restore: Read(%q) error = %v, want live audit", aud.ID, err)
				}
				if !tt.live && !errors.Is(err, ErrNotFound) {
					t.Errorf("after rejected Restore: Read(%q) error = %v, want ErrNotFound", aud.ID, err)
				}
				return
			}
			if restored.ID != aud.ID {
				t.Errorf("Restore().ID = %q, want %q", restored.ID, aud.ID)
			}

			// Should be readable again.
			read, err := store.Read(ctx, aud.ID)
			if err != nil {
				t.Fatalf("after Restore: Read(%q) error = %v", aud.ID, err)
			}
			if read.Content != "Review." {
				t.Errorf("after Restore: Read(%q).Content = %q, want %q", aud.ID, read.Content, "Review.")
			}
		})
	}
}

// TestStatus covers the agent inbox: only threads whose effective
// assignee matches and whose effective status is pending or
// needs-work are counted. Two fixtures are used: three top-level
// audits with different assignees, and a thread reassigned by reply.
func TestStatus(t *testing.T) {
	assigned := func(t *testing.T, store *Audits) {
		t.Helper()
		ctx := t.Context()
		for _, opts := range []AddOptions{
			// Gemini creates audit assigned to claude-code.
			{Target: "docs/api", Content: "Needs work.", Author: "gemini", Assignee: "claude-code", Status: "needs-work"},
			// Claude-code creates audit assigned to gemini.
			{Target: "docs/auth", Content: "LGTM.", Author: "claude-code", Assignee: "gemini", Status: "pending"},
			// Unassigned audit - should not appear in anyone's status.
			{Target: "docs/config", Content: "FYI.", Author: "gemini", Status: "pending"},
		} {
			if _, err := store.Add(ctx, opts); err != nil {
				t.Fatalf("Add(%q) error = %v", opts.Target, err)
			}
		}
	}
	reassigned := func(t *testing.T, store *Audits) {
		t.Helper()
		ctx := t.Context()
		// Gemini creates audit assigned to claude-code.
		aud, err := store.Add(ctx, AddOptions{Target: "docs/api", Content: "Review.", Author: "gemini", Assignee: "claude-code"})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		// Claude-code replies, reassigning to gemini.
		if _, err := store.Reply(ctx, aud.ID, AddOptions{Content: "Done, your turn.", Author: "claude-code", Assignee: "gemini"}); err != nil {
			t.Fatalf("Reply() error = %v", err)
		}
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T, store *Audits)
		author  string
		wantErr error
		want    Summary
	}{
		{"needs-work inbox", assigned, "claude-code", nil, Summary{Total: 1, NeedsWork: 1}},
		{"pending inbox", assigned, "gemini", nil, Summary{Total: 1, Pending: 1}},
		{"nothing assigned", assigned, "nobody", nil, Summary{}},
		{"reassigned away", reassigned, "claude-code", nil, Summary{}},
		{"reassigned to", reassigned, "gemini", nil, Summary{Total: 1, Pending: 1}},
		{"missing author", assigned, "", ErrMissingAuthor, Summary{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			tt.setup(t, store)

			result, err := store.Status(t.Context(), tt.author, 0)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Status(%q) error = %v, want %v", tt.author, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if result.Summary != tt.want {
				t.Errorf("Status(%q).Summary = %+v, want %+v", tt.author, result.Summary, tt.want)
			}
			if len(result.Pending) != tt.want.Total {
				t.Errorf("len(Status(%q).Pending) = %d, want %d", tt.author, len(result.Pending), tt.want.Total)
			}
		})
	}
}

func TestListSince(t *testing.T) {
	store := newTestStore(t)
	ctx := t.Context()

	old, _ := store.Add(ctx, AddOptions{
		Target: "docs/old", Content: "Old.", Author: "alice",
	})

	cutoff := old.CreatedAt

	// Ensure the next audit gets a different millisecond timestamp.
	time.Sleep(2 * time.Millisecond)

	if _, err := store.Add(ctx, AddOptions{
		Target: "docs/new", Content: "New.", Author: "alice",
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Since = cutoff should return only the newer audit.
	recent, err := store.List(ctx, ListOptions{SinceMS: cutoff})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent audit, got %d", len(recent))
	}
	if recent[0].Target != "docs/new" {
		t.Errorf("target = %q, want %q", recent[0].Target, "docs/new")
	}
}
