package host

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jpl-au/llmd/engine/audits"
	"github.com/jpl-au/llmd/sdk"
)

// addAudit creates an audit and fails the test on error.
func addAudit(t *testing.T, c sdk.Context, opts sdk.AuditOpts) *sdk.Audit {
	t.Helper()
	aud, err := c.Audits.Add(opts)
	if err != nil {
		t.Fatalf("Audits.Add(%+v) error = %v", opts, err)
	}
	return aud
}

// TestAuditsList seeds three threads and checks every filter. The
// fixture, oldest first:
//
//	0: alice on docs/a, assigned to bob, pending
//	1: bob on docs/b, assigned to alice, needs-work
//	2: alice on docs/a, assigned to carol, then approved by carol
//
// Results are newest first, so want holds fixture indexes in that
// order.
func TestAuditsList(t *testing.T) {
	tests := []struct {
		name string
		opts func(now time.Time) sdk.AuditListOpts
		want []int
	}{
		{"all", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{} }, []int{2, 1, 0}},
		{"target", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Target: "docs/a"} }, []int{2, 0}},
		{"by author", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{ByAuthor: "bob"} }, []int{1}},
		{"assignee", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Assignee: "bob"} }, []int{0}},
		{"status pending", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Status: "pending"} }, []int{0}},
		{"status approved", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Status: "approved"} }, []int{2}},
		{"status with target", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Status: "approved", Target: "docs/b"} }, nil},
		{"pending shorthand", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Pending: true} }, []int{1, 0}},
		{"pending by author", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Pending: true, ByAuthor: "alice"} }, []int{0}},
		{"pending by assignee", func(time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Pending: true, Assignee: "alice"} }, []int{1}},
		{"since the past", func(now time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Since: now.Add(-time.Minute)} }, []int{2, 1, 0}},
		{"since the future", func(now time.Time) sdk.AuditListOpts { return sdk.AuditListOpts{Since: now.Add(time.Minute)} }, nil},
		{"pending since the future", func(now time.Time) sdk.AuditListOpts {
			return sdk.AuditListOpts{Pending: true, Since: now.Add(time.Minute)}
		}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			now := time.Now()
			ids := []string{
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "first", Author: "alice", Assignee: "bob"}).ID,
				addAudit(t, c, sdk.AuditOpts{Target: "docs/b", Content: "second", Author: "bob", Assignee: "alice", Status: "needs-work"}).ID,
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "third", Author: "alice", Assignee: "carol"}).ID,
			}
			if _, err := c.Audits.Resolve(ids[2], "carol"); err != nil {
				t.Fatalf("Resolve: %v", err)
			}

			opts := tt.opts(now)
			got, err := c.Audits.List(opts)
			if err != nil {
				t.Fatalf("List(%+v) error = %v", opts, err)
			}
			var gotIDs, wantIDs []string
			for _, a := range got {
				gotIDs = append(gotIDs, a.ID)
			}
			for _, i := range tt.want {
				wantIDs = append(wantIDs, ids[i])
			}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Errorf("List(%+v) = %v, want %v (fixture %v)", opts, gotIDs, wantIDs, ids)
			}
		})
	}
}

// TestAuditsStatus covers the inbox: which threads need the author's
// attention, the needs-work/pending split, and the errors.
func TestAuditsStatus(t *testing.T) {
	tests := []struct {
		name    string
		seed    func(t *testing.T, c sdk.Context)
		author  string
		opts    func(now time.Time) sdk.AuditStatusOpts
		want    sdk.AuditSummary
		wantErr error
	}{
		{
			name:   "empty inbox",
			seed:   func(*testing.T, sdk.Context) {},
			author: "bob",
		},
		{
			name: "pending thread assigned to the author",
			seed: func(t *testing.T, c sdk.Context) {
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
			},
			author: "bob",
			want:   sdk.AuditSummary{Total: 1, Pending: 1},
		},
		{
			name: "needs-work is counted separately",
			seed: func(t *testing.T, c sdk.Context) {
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "fix", Author: "alice", Assignee: "bob", Status: "needs-work"})
				addAudit(t, c, sdk.AuditOpts{Target: "docs/b", Content: "look", Author: "alice", Assignee: "bob"})
			},
			author: "bob",
			want:   sdk.AuditSummary{Total: 2, NeedsWork: 1, Pending: 1},
		},
		{
			name: "thread assigned to someone else",
			seed: func(t *testing.T, c sdk.Context) {
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "carol"})
			},
			author: "bob",
		},
		{
			name: "reply reassigns the thread",
			seed: func(t *testing.T, c sdk.Context) {
				aud := addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
				if _, err := c.Audits.Reply(aud.ID, sdk.AuditOpts{Content: "done?", Author: "bob", Assignee: "alice"}); err != nil {
					t.Fatalf("Reply: %v", err)
				}
			},
			author: "alice",
			want:   sdk.AuditSummary{Total: 1, Pending: 1},
		},
		{
			name: "resolved thread leaves the inbox",
			seed: func(t *testing.T, c sdk.Context) {
				aud := addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
				if _, err := c.Audits.Resolve(aud.ID, "bob"); err != nil {
					t.Fatalf("Resolve: %v", err)
				}
			},
			author: "bob",
		},
		{
			name: "since the future",
			seed: func(t *testing.T, c sdk.Context) {
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
			},
			author: "bob",
			opts:   func(now time.Time) sdk.AuditStatusOpts { return sdk.AuditStatusOpts{Since: now.Add(time.Minute)} },
		},
		{
			name: "since the past",
			seed: func(t *testing.T, c sdk.Context) {
				addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
			},
			author: "bob",
			opts:   func(now time.Time) sdk.AuditStatusOpts { return sdk.AuditStatusOpts{Since: now.Add(-time.Minute)} },
			want:   sdk.AuditSummary{Total: 1, Pending: 1},
		},
		{
			name:    "missing author",
			seed:    func(*testing.T, sdk.Context) {},
			author:  "",
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			now := time.Now()
			tt.seed(t, c)
			var opts sdk.AuditStatusOpts
			if tt.opts != nil {
				opts = tt.opts(now)
			}

			got, err := c.Audits.Status(tt.author, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Status(%q, %+v) error = %v, want %v", tt.author, opts, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Author != tt.author {
				t.Errorf("Status(%q).Author = %q", tt.author, got.Author)
			}
			if got.Summary != tt.want {
				t.Errorf("Status(%q, %+v).Summary = %+v, want %+v", tt.author, opts, got.Summary, tt.want)
			}
			if len(got.Pending) != tt.want.Total {
				t.Errorf("Status(%q, %+v) has %d pending, want %d", tt.author, opts, len(got.Pending), tt.want.Total)
			}
		})
	}
}

// TestAuditsRead checks the fields of a created audit, including the
// inferred target type and the pinned document version. An audit that
// pins no version records zero, as sdk.Audit.Version documents.
func TestAuditsRead(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, c sdk.Context) string // returns the target
		opts  sdk.AuditOpts
		want  sdk.Audit // ID, Target and CreatedAt are filled in
	}{
		{
			name: "document target without a pinned version",
			setup: func(t *testing.T, c sdk.Context) string {
				writeDoc(t, c, "docs/a", "v1")
				writeDoc(t, c, "docs/a", "v2")
				return "docs/a"
			},
			opts: sdk.AuditOpts{Content: "check this", Author: "alice", Assignee: "bob"},
			want: sdk.Audit{TargetType: "document", Author: "alice", Assignee: "bob", Status: "pending", Content: "check this"},
		},
		{
			name: "pinned version and status",
			setup: func(t *testing.T, c sdk.Context) string {
				writeDoc(t, c, "docs/a", "v1")
				writeDoc(t, c, "docs/a", "v2")
				return "docs/a"
			},
			opts: sdk.AuditOpts{Content: "older", Author: "alice", Version: 1, Status: "needs-work"},
			want: sdk.Audit{TargetType: "document", Version: 1, Author: "alice", Status: "needs-work", Content: "older"},
		},
		{
			name: "task target",
			setup: func(t *testing.T, c sdk.Context) string {
				return addTask(t, c, "Audited", spec, sdk.TaskAddOpts{Author: "alice"}).Key
			},
			opts: sdk.AuditOpts{Content: "review", Author: "bob"},
			want: sdk.Audit{TargetType: "task", Author: "bob", Status: "pending", Content: "review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			target := tt.setup(t, c)
			opts := tt.opts
			opts.Target = target
			created := addAudit(t, c, opts)

			got, err := c.Audits.Read(created.ID)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", created.ID, err)
			}
			want := tt.want
			want.ID, want.Target, want.CreatedAt = created.ID, target, got.CreatedAt
			if got.CreatedAt == 0 {
				t.Errorf("Read(%q).CreatedAt = 0, want a timestamp", created.ID)
			}
			if *got != want {
				t.Errorf("Read(%q) = %+v, want %+v", created.ID, *got, want)
			}
			if *created != want {
				t.Errorf("Add(%+v) = %+v, want %+v", opts, *created, want)
			}
		})
	}
}

// TestAuditsResolve covers the approved entry a resolve appends and
// the author and not-found errors.
func TestAuditsResolve(t *testing.T) {
	tests := []struct {
		name    string
		id      func(parent string) string
		author  string
		wantErr error
	}{
		{"resolves", func(p string) string { return p }, "bob", nil},
		{"missing author", func(p string) string { return p }, "", sdk.ErrMissingArg},
		{"missing thread", func(string) string { return "missing" }, "bob", sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			parent := addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice", Assignee: "bob"})
			id := tt.id(parent.ID)

			got, err := c.Audits.Resolve(id, tt.author)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Resolve(%q, %q) error = %v, want %v", id, tt.author, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Status != "approved" || got.ParentID != parent.ID || got.Author != tt.author || got.Content != "" || got.Target != "docs/a" {
				t.Errorf("Resolve(%q, %q) = %+v, want an empty approved reply by %s on %s", id, tt.author, got, tt.author, parent.ID)
			}
			thread, err := c.Audits.Thread(parent.ID)
			if err != nil {
				t.Fatalf("Thread(%q) error = %v", parent.ID, err)
			}
			if len(thread) != 2 || thread[1].ID != got.ID {
				t.Errorf("Thread(%q) = %+v, want the parent then the resolve entry", parent.ID, thread)
			}
			listed, err := c.Audits.List(sdk.AuditListOpts{Status: "approved"})
			if err != nil {
				t.Fatalf("List(approved) error = %v", err)
			}
			if len(listed) != 1 || listed[0].ID != parent.ID {
				t.Errorf("List(approved) = %+v, want the resolved thread", listed)
			}
		})
	}
}

// TestAuditsRestore covers restoring a deleted audit and the errors
// for a live audit and a missing author.
func TestAuditsRestore(t *testing.T) {
	tests := []struct {
		name    string
		delete  bool
		author  string
		wantErr error
	}{
		{"restores a deleted audit", true, "bob", nil},
		{"live audit is not found", false, "bob", sdk.ErrNotFound},
		{"missing author", true, "", sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			aud := addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
			if tt.delete {
				if err := c.Audits.Delete(aud.ID, "alice"); err != nil {
					t.Fatalf("Delete: %v", err)
				}
				if _, err := c.Audits.Read(aud.ID); !errors.Is(err, sdk.ErrNotFound) {
					t.Fatalf("Read after Delete error = %v, want %v", err, sdk.ErrNotFound)
				}
			}

			got, err := c.Audits.Restore(aud.ID, tt.author)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Restore(%q, %q) error = %v, want %v", aud.ID, tt.author, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.ID != aud.ID || got.Content != "look" {
				t.Errorf("Restore(%q) = %+v, want the original audit", aud.ID, got)
			}
			if _, err := c.Audits.Read(aud.ID); err != nil {
				t.Errorf("Read after Restore error = %v, want nil", err)
			}
		})
	}
}

// TestAuditsAddErrors covers the argument errors of Add and Reply.
func TestAuditsAddErrors(t *testing.T) {
	tests := []struct {
		name    string
		call    func(c sdk.Context, parent string) error
		wantErr error
	}{
		{"add without target", func(c sdk.Context, _ string) error {
			_, err := c.Audits.Add(sdk.AuditOpts{Content: "x", Author: "alice"})
			return err
		}, sdk.ErrMissingArg},
		{"add without author", func(c sdk.Context, _ string) error {
			_, err := c.Audits.Add(sdk.AuditOpts{Target: "docs/a", Content: "x"})
			return err
		}, sdk.ErrMissingArg},
		{"reply without author", func(c sdk.Context, parent string) error {
			_, err := c.Audits.Reply(parent, sdk.AuditOpts{Content: "x"})
			return err
		}, sdk.ErrMissingArg},
		{"delete without author", func(c sdk.Context, parent string) error {
			return c.Audits.Delete(parent, "")
		}, sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			parent := addAudit(t, c, sdk.AuditOpts{Target: "docs/a", Content: "look", Author: "alice"})
			if err := tt.call(c, parent.ID); !errors.Is(err, tt.wantErr) {
				t.Errorf("%s error = %v, want %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// TestAuditErr covers the translation of every audits sentinel.
func TestAuditErr(t *testing.T) {
	other := errors.New("disk on fire")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"not found", audits.ErrNotFound, sdk.ErrNotFound},
		{"missing author", audits.ErrMissingAuthor, sdk.ErrMissingArg},
		{"missing target", audits.ErrMissingTarget, sdk.ErrMissingArg},
		{"passthrough", other, other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auditErr(tt.in)
			if !errors.Is(got, tt.want) {
				t.Errorf("auditErr(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.in != nil && !errors.Is(got, tt.in) {
				t.Errorf("auditErr(%v) = %v, want to wrap the original", tt.in, got)
			}
		})
	}
}
