package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// auditOf returns the sdk.Audit carried by an audit add / reply /
// resolve response, failing the test if the payload has another shape.
func auditOf(t *testing.T, resp sdk.Response) sdk.Audit {
	t.Helper()
	a, ok := data(resp).(*sdk.Audit)
	if !ok || a == nil {
		t.Fatalf("audit response Data = %T, want *sdk.Audit", data(resp))
	}
	return *a
}

// auditsOf returns the []sdk.Audit carried by an audit list / show
// response.
func auditsOf(t *testing.T, resp sdk.Response) []sdk.Audit {
	t.Helper()
	aa, ok := data(resp).([]sdk.Audit)
	if !ok {
		t.Fatalf("audit response Data = %T, want []sdk.Audit", data(resp))
	}
	return aa
}

// auditCreate creates a top-level audit as c's author and returns it.
func auditCreate(t *testing.T, c sdk.Context, target, content string, args ...string) sdk.Audit {
	t.Helper()
	all := append([]string{"add", target, content}, args...)
	return auditOf(t, run(t, c, "audit", nil, all...))
}

// auditRespond replies to id as c's author and returns the new entry.
func auditRespond(t *testing.T, c sdk.Context, id, content string, args ...string) sdk.Audit {
	t.Helper()
	all := append([]string{"reply", id, content}, args...)
	return auditOf(t, run(t, c, "audit", nil, all...))
}

// auditTask creates a task with a spec body and returns its key, so
// audits can target a task and exercise target type inference.
func auditTask(t *testing.T, c sdk.Context, title string) string {
	t.Helper()
	task, err := c.Tasks.Add(title, []byte("# "+title+"\n\nBody."), sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Tasks.Add(%q): %v", title, err)
	}
	return task.Key
}

// auditExpand replaces fixture placeholders in args.
func auditExpand(args []string, repl *strings.Replacer) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = repl.Replace(a)
	}
	return out
}

// TestAuditCmd covers the dispatcher: subcommand required, unknown
// subcommands rejected, and the author rule for a mixed command (reads
// work without one, mutations and status do not).
func TestAuditCmd(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		args    []string
		wantErr error  // sentinel, when one applies
		wantMsg string // substring of the error, when no sentinel applies
	}{
		{name: "no subcommand", author: "alice", wantErr: sdk.ErrMissingArg},
		{name: "unknown subcommand", author: "alice", args: []string{"bogus"}, wantMsg: "unknown subcommand"},
		{name: "add needs author", args: []string{"add", "docs/a", "x"}, wantMsg: "an author is required."},
		{name: "reply needs author", args: []string{"reply", "abc", "x"}, wantMsg: "an author is required."},
		{name: "resolve needs author", args: []string{"resolve", "abc"}, wantMsg: "an author is required."},
		{name: "rm needs author", args: []string{"rm", "abc"}, wantMsg: "an author is required."},
		{name: "restore needs author", args: []string{"restore", "abc"}, wantMsg: "an author is required."},
		{name: "status needs author", args: []string{"status"}, wantMsg: "an author is required."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, tt.author)
			err := runErr(t, c, "audit", nil, tt.args...)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("audit %v error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
				t.Errorf("audit %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
			}
		})
	}

	// list and show are reads: they must work without an author.
	t.Run("list and show without author", func(t *testing.T) {
		c := newCLI(t, "alice")
		id := auditCreate(t, c, "docs/a", "review me").ID
		noAuthor := c
		noAuthor.Author = ""
		if got := text(run(t, noAuthor, "audit", nil, "list")); !strings.Contains(got, id) {
			t.Errorf("audit list without author = %q, want to contain %q", got, id)
		}
		if got := text(run(t, noAuthor, "audit", nil, "show", id)); !strings.Contains(got, "review me") {
			t.Errorf("audit show without author = %q, want to contain %q", got, "review me")
		}
	})
}

// TestAuditAdd covers content resolution (positional, then --file,
// then stdin), the status / assign / version flags, target type
// inference and the argument errors. Fixtures: docs/a exists, {task}
// is a task key and {file} is a file holding "from file".
func TestAuditAdd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantText   []string
		want       sdk.Audit // fields compared when non-zero
		wantErr    error
		wantErrMsg string
	}{
		{
			name:     "inline content",
			args:     []string{"add", "docs/a", "Needs review."},
			wantText: []string{"Created audit ", " on docs/a [pending]"},
			want:     sdk.Audit{Target: "docs/a", TargetType: "document", Author: "alice", Status: "pending", Content: "Needs review."},
		},
		{
			name: "positional words are joined",
			args: []string{"add", "docs/a", "two", "words"},
			want: sdk.Audit{Content: "two words"},
		},
		{
			name: "content from --file",
			args: []string{"add", "docs/a", "--file", "{file}"},
			want: sdk.Audit{Content: "from file"},
		},
		{
			name:  "content from stdin",
			args:  []string{"add", "docs/a"},
			stdin: "from stdin",
			want:  sdk.Audit{Content: "from stdin"},
		},
		{
			name:  "positional beats --file and stdin",
			args:  []string{"add", "docs/a", "inline", "--file", "{file}"},
			stdin: "from stdin",
			want:  sdk.Audit{Content: "inline"},
		},
		{
			name:  "--file beats stdin",
			args:  []string{"add", "docs/a", "--file", "{file}"},
			stdin: "from stdin",
			want:  sdk.Audit{Content: "from file"},
		},
		{
			name: "no content at all is allowed",
			args: []string{"add", "docs/a"},
			want: sdk.Audit{Target: "docs/a", Status: "pending"},
		},
		{
			name:     "status assign and version",
			args:     []string{"add", "docs/a", "x", "--status", "needs-work", "--assign", "bob", "--version", "2"},
			wantText: []string{"[needs-work]"},
			want:     sdk.Audit{Status: "needs-work", Assignee: "bob", Version: 2},
		},
		{
			name: "short status flag",
			args: []string{"add", "docs/a", "x", "-s", "info"},
			want: sdk.Audit{Status: "info"},
		},
		{
			name: "task target is inferred",
			args: []string{"add", "{task}", "looks wrong"},
			want: sdk.Audit{TargetType: "task"},
		},
		{
			name: "unknown target path is still accepted",
			args: []string{"add", "docs/missing", "x"},
			want: sdk.Audit{Target: "docs/missing", TargetType: "document"},
		},
		{
			name:    "no target",
			args:    []string{"add"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"add", "docs/a", "x", "--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "version must be an integer",
			args:    []string{"add", "docs/a", "x", "--version", "two"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "flag without value",
			args:    []string{"add", "docs/a", "x", "--assign"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "missing --file",
			args:    []string{"add", "docs/a", "--file", "{dir}/nope.md"},
			wantErr: fs.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# A")
			dir := t.TempDir()
			file := filepath.Join(dir, "review.md")
			if err := os.WriteFile(file, []byte("from file"), 0o644); err != nil {
				t.Fatal(err)
			}
			task := auditTask(t, c, "Audit target")
			args := auditExpand(tt.args, strings.NewReplacer("{file}", file, "{dir}", dir, "{task}", task))

			var stdin []byte
			if tt.stdin != "" {
				stdin = []byte(tt.stdin)
			}

			if tt.wantErr != nil || tt.wantErrMsg != "" {
				err := runErr(t, c, "audit", stdin, args...)
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				if tt.wantErrMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrMsg)) {
					t.Errorf("audit %v error = %v, want to contain %q", args, err, tt.wantErrMsg)
				}
				return
			}

			resp := run(t, c, "audit", stdin, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, want)
				}
			}
			aud := auditOf(t, resp)
			if aud.ID == "" || len(aud.ID) != 10 {
				t.Errorf("audit %v ID = %q, want a 10-char key", args, aud.ID)
			}
			if aud.ParentID != "" {
				t.Errorf("audit %v ParentID = %q, want empty for a top-level audit", args, aud.ParentID)
			}
			auditCompare(t, args, aud, tt.want)

			// The audit is readable back through show.
			if got := text(run(t, c, "audit", nil, "show", aud.ID)); !strings.Contains(got, aud.ID) {
				t.Errorf("audit show %s = %q, want to contain the id", aud.ID, got)
			}
		})
	}
}

// auditCompare checks every non-zero field of want against got.
func auditCompare(t *testing.T, args []string, got, want sdk.Audit) {
	t.Helper()
	if want.Target != "" && got.Target != want.Target {
		t.Errorf("audit %v Target = %q, want %q", args, got.Target, want.Target)
	}
	if want.TargetType != "" && got.TargetType != want.TargetType {
		t.Errorf("audit %v TargetType = %q, want %q", args, got.TargetType, want.TargetType)
	}
	if want.Author != "" && got.Author != want.Author {
		t.Errorf("audit %v Author = %q, want %q", args, got.Author, want.Author)
	}
	if want.Assignee != "" && got.Assignee != want.Assignee {
		t.Errorf("audit %v Assignee = %q, want %q", args, got.Assignee, want.Assignee)
	}
	if want.Status != "" && got.Status != want.Status {
		t.Errorf("audit %v Status = %q, want %q", args, got.Status, want.Status)
	}
	if want.Content != "" && got.Content != want.Content {
		t.Errorf("audit %v Content = %q, want %q", args, got.Content, want.Content)
	}
	if want.Version != 0 && got.Version != want.Version {
		t.Errorf("audit %v Version = %d, want %d", args, got.Version, want.Version)
	}
	if want.ParentID != "" && got.ParentID != want.ParentID {
		t.Errorf("audit %v ParentID = %q, want %q", args, got.ParentID, want.ParentID)
	}
}

// TestAuditReply covers replies: status and assignee inheritance,
// overriding both, content resolution, flattening of nested replies
// and the error cases. {root} is a pending audit on docs/a assigned to
// bob; {reply} is an existing reply to it.
func TestAuditReply(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantText []string
		want     sdk.Audit
		wantErr  error
	}{
		{
			name:     "inherits status and assignee",
			args:     []string{"reply", "{root}", "Fixed."},
			wantText: []string{"Replied to {root} [pending]"},
			want:     sdk.Audit{ParentID: "{root}", Status: "pending", Assignee: "bob", Author: "carol", Content: "Fixed.", Target: "docs/a"},
		},
		{
			name:     "status override resolves",
			args:     []string{"reply", "{root}", "--status", "approved"},
			wantText: []string{"[approved]"},
			want:     sdk.Audit{Status: "approved", ParentID: "{root}"},
		},
		{
			name: "short status flag",
			args: []string{"reply", "{root}", "x", "-s", "needs-work"},
			want: sdk.Audit{Status: "needs-work"},
		},
		{
			name: "reassign",
			args: []string{"reply", "{root}", "Please check.", "--assign", "gemini"},
			want: sdk.Audit{Assignee: "gemini"},
		},
		{
			name: "content from --file",
			args: []string{"reply", "{root}", "--file", "{file}"},
			want: sdk.Audit{Content: "from file"},
		},
		{
			name:  "content from stdin",
			args:  []string{"reply", "{root}"},
			stdin: "from stdin",
			want:  sdk.Audit{Content: "from stdin"},
		},
		{
			name:  "positional beats stdin",
			args:  []string{"reply", "{root}", "inline"},
			stdin: "from stdin",
			want:  sdk.Audit{Content: "inline"},
		},
		{
			name:     "reply to a reply flattens to the root",
			args:     []string{"reply", "{reply}", "nested"},
			wantText: []string{"Replied to {root}"},
			want:     sdk.Audit{ParentID: "{root}"},
		},
		{
			name:    "missing id",
			args:    []string{"reply"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown id",
			args:    []string{"reply", "zzzzzzzzzz", "x"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "unknown flag",
			args:    []string{"reply", "{root}", "x", "--version", "1"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# A")
			root := auditCreate(t, c, "docs/a", "Needs work.", "--assign", "bob")
			reply := auditRespond(t, c, root.ID, "first reply")
			file := filepath.Join(t.TempDir(), "reply.md")
			if err := os.WriteFile(file, []byte("from file"), 0o644); err != nil {
				t.Fatal(err)
			}
			repl := strings.NewReplacer("{root}", root.ID, "{reply}", reply.ID, "{file}", file)
			args := auditExpand(tt.args, repl)

			var stdin []byte
			if tt.stdin != "" {
				stdin = []byte(tt.stdin)
			}

			carol := c
			carol.Author = "carol"
			if tt.wantErr != nil {
				if err := runErr(t, carol, "audit", stdin, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, carol, "audit", stdin, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if want = repl.Replace(want); !strings.Contains(got, want) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, want)
				}
			}
			want := tt.want
			want.ParentID = repl.Replace(want.ParentID)
			aud := auditOf(t, resp)
			auditCompare(t, args, aud, want)
			if aud.ID == root.ID || aud.ID == reply.ID {
				t.Errorf("audit %v returned an existing id %q", args, aud.ID)
			}

			// The new entry is part of the root's thread.
			thread := auditsOf(t, run(t, c, "audit", nil, "show", root.ID))
			if thread[len(thread)-1].ID != aud.ID {
				t.Errorf("audit show %s last entry = %q, want the new reply %q", root.ID, thread[len(thread)-1].ID, aud.ID)
			}
		})
	}
}

// TestAuditList covers listing and every filter. Fixture (newest
// last): {a1} pending on docs/a by alice assigned to bob; {a2} on
// docs/b by dave assigned to carol, effective status needs-work after a
// reply; {a3} on docs/a by alice, resolved.
func TestAuditList(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantIDs    []string // placeholders that must appear, in order
		wantAbsent []string
		wantEmpty  bool
		wantErr    error
	}{
		{
			name:    "all top-level audits newest first",
			args:    []string{"list"},
			wantIDs: []string{"{a3}", "{a2}", "{a1}"},
		},
		{
			name:    "ls alias",
			args:    []string{"ls"},
			wantIDs: []string{"{a3}", "{a2}", "{a1}"},
		},
		{
			name:       "filter by target",
			args:       []string{"list", "docs/b"},
			wantIDs:    []string{"{a2}"},
			wantAbsent: []string{"{a1}", "{a3}"},
		},
		{
			name:       "filter by author",
			args:       []string{"list", "--by-author", "dave"},
			wantIDs:    []string{"{a2}"},
			wantAbsent: []string{"{a1}", "{a3}"},
		},
		{
			name:       "filter by assignee",
			args:       []string{"list", "--assign", "bob"},
			wantIDs:    []string{"{a1}"},
			wantAbsent: []string{"{a2}", "{a3}"},
		},
		{
			name:       "filter by effective status",
			args:       []string{"list", "--status", "needs-work"},
			wantIDs:    []string{"{a2}"},
			wantAbsent: []string{"{a1}", "{a3}"},
		},
		{
			name:       "short status flag uses effective status",
			args:       []string{"list", "-s", "pending"},
			wantIDs:    []string{"{a1}"},
			wantAbsent: []string{"{a2}", "{a3}"},
		},
		{
			name:       "pending excludes resolved threads",
			args:       []string{"list", "--pending"},
			wantIDs:    []string{"{a2}", "{a1}"},
			wantAbsent: []string{"{a3}"},
		},
		{
			name:       "pending combined with target",
			args:       []string{"list", "--pending", "docs/a"},
			wantIDs:    []string{"{a1}"},
			wantAbsent: []string{"{a2}", "{a3}"},
		},
		{
			name:    "since a duration includes recent audits",
			args:    []string{"list", "--since", "1h"},
			wantIDs: []string{"{a3}", "{a2}", "{a1}"},
		},
		{
			name:      "since a future timestamp is empty",
			args:      []string{"list", "--since", "2100-01-01T00:00:00Z"},
			wantEmpty: true,
		},
		{
			name:      "no match",
			args:      []string{"list", "docs/none"},
			wantEmpty: true,
		},
		{
			name:    "invalid since",
			args:    []string{"list", "--since", "yesterday"},
			wantErr: errAuditAny,
		},
		{
			name:    "unknown flag",
			args:    []string{"list", "--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Distinct, increasing timestamps so "newest first" is
			// well defined; the list query orders by created_at only.
			clocktest.Sequence(t, time.Now().Add(-10*time.Minute).UnixMilli(), 1000)
			c := newCLI(t, "alice")
			a1 := auditCreate(t, c, "docs/a", "first", "--assign", "bob")
			dave := c
			dave.Author = "dave"
			a2 := auditCreate(t, dave, "docs/b", "second", "--assign", "carol")
			auditRespond(t, c, a2.ID, "please fix", "--status", "needs-work")
			a3 := auditCreate(t, c, "docs/a", "third")
			run(t, c, "audit", nil, "resolve", a3.ID)
			repl := strings.NewReplacer("{a1}", a1.ID, "{a2}", a2.ID, "{a3}", a3.ID)
			args := auditExpand(tt.args, repl)

			// Reads need no author.
			anon := c
			anon.Author = ""

			if tt.wantErr != nil {
				err := runErr(t, anon, "audit", nil, args...)
				if tt.wantErr == errAuditAny && err == nil {
					t.Errorf("audit %v error = nil, want an error", args)
				} else if tt.wantErr != errAuditAny && !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, anon, "audit", nil, args...)
			got := text(resp)
			list := auditsOf(t, resp)
			if tt.wantEmpty {
				if got != "No audits" || len(list) != 0 {
					t.Errorf("audit %v = %q (%d audits), want %q and none", args, got, len(list), "No audits")
				}
				return
			}
			if len(list) != len(tt.wantIDs) {
				t.Errorf("audit %v returned %d audits, want %d", args, len(list), len(tt.wantIDs))
			}
			for i, ph := range tt.wantIDs {
				id := repl.Replace(ph)
				if !strings.Contains(got, id) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, id)
				}
				if i < len(list) && list[i].ID != id {
					t.Errorf("audit %v Data[%d].ID = %q, want %q", args, i, list[i].ID, id)
				}
			}
			for _, ph := range tt.wantAbsent {
				if id := repl.Replace(ph); strings.Contains(got, id) {
					t.Errorf("audit %v = %q, must not contain %q", args, got, id)
				}
			}
			// Each line carries target, status and author.
			for _, a := range list {
				for _, col := range []string{a.Target, a.Status, a.Author} {
					if !strings.Contains(got, col) {
						t.Errorf("audit %v = %q, want to contain %q", args, got, col)
					}
				}
			}
		})
	}
}

// errAuditAny marks a table case that wants some error without a sentinel.
var errAuditAny = errors.New("any error")

// TestAuditShow covers thread rendering, reply resolution to the root,
// the default cap of ten entries with --all to lift it, and the error
// cases. {root} has {n} replies.
func TestAuditShow(t *testing.T) {
	tests := []struct {
		name       string
		replies    int
		args       []string
		wantText   []string
		wantAbsent []string
		wantLen    int
		wantErr    error
	}{
		{
			name:     "root only",
			args:     []string{"show", "{root}"},
			wantText: []string{"alice", "[pending]", "{root}", "Needs review."},
			wantLen:  1,
		},
		{
			name:     "replies are indented under the root",
			replies:  2,
			args:     []string{"show", "{root}"},
			wantText: []string{"Needs review.", "└ bob", "    reply 1", "    reply 2"},
			wantLen:  3,
		},
		{
			name:     "reply id resolves to the whole thread",
			replies:  2,
			args:     []string{"show", "{last}"},
			wantText: []string{"Needs review.", "reply 1", "reply 2"},
			wantLen:  3,
		},
		{
			name:       "long thread is capped at ten",
			replies:    12,
			args:       []string{"show", "{root}"},
			wantText:   []string{"Needs review.", "3 messages hidden", "--all", "reply 4", "reply 12"},
			wantAbsent: []string{"reply 1\n", "reply 2\n", "reply 3\n"},
			wantLen:    10,
		},
		{
			name:       "--all lifts the cap",
			replies:    12,
			args:       []string{"show", "--all", "{root}"},
			wantText:   []string{"reply 1", "reply 3", "reply 12"},
			wantAbsent: []string{"hidden"},
			wantLen:    13,
		},
		{
			name:    "missing id",
			args:    []string{"show"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown id",
			args:    []string{"show", "zzzzzzzzzz"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "unknown flag",
			args:    []string{"show", "--pending", "{root}"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			root := auditCreate(t, c, "docs/a", "Needs review.")
			bob := c
			bob.Author = "bob"
			last := root.ID
			for i := 1; i <= tt.replies; i++ {
				last = auditRespond(t, bob, root.ID, "reply "+strconv.Itoa(i)).ID
			}
			repl := strings.NewReplacer("{root}", root.ID, "{last}", last)
			args := auditExpand(tt.args, repl)

			anon := c
			anon.Author = ""
			if tt.wantErr != nil {
				if err := runErr(t, anon, "audit", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, anon, "audit", nil, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if want = repl.Replace(want); !strings.Contains(got, want) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("audit %v = %q, must not contain %q", args, got, absent)
				}
			}
			thread := auditsOf(t, resp)
			if len(thread) != tt.wantLen {
				t.Errorf("audit %v Data has %d entries, want %d", args, len(thread), tt.wantLen)
			}
			if len(thread) > 0 && thread[0].ID != root.ID {
				t.Errorf("audit %v Data[0].ID = %q, want the root %q", args, thread[0].ID, root.ID)
			}
			if len(thread) > 1 && thread[len(thread)-1].ID != last {
				t.Errorf("audit %v last entry = %q, want the newest reply %q", args, thread[len(thread)-1].ID, last)
			}
		})
	}

	t.Run("empty content is marked", func(t *testing.T) {
		c := newCLI(t, "alice")
		root := auditCreate(t, c, "docs/a", "x")
		run(t, c, "audit", nil, "resolve", root.ID)
		if got := text(run(t, c, "audit", nil, "show", root.ID)); !strings.Contains(got, "(no content)") {
			t.Errorf("audit show = %q, want to contain %q", got, "(no content)")
		}
	})
}

// TestAuditResolve covers the approval shortcut: it inserts an
// "approved" reply with no content, which drops the thread from the
// pending list.
func TestAuditResolve(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "resolves the thread", args: []string{"resolve", "{root}"}},
		{name: "resolve via a reply id", args: []string{"resolve", "{reply}"}},
		{name: "missing id", args: []string{"resolve"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown id", args: []string{"resolve", "zzzzzzzzzz"}, wantErr: sdk.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			root := auditCreate(t, c, "docs/a", "x", "--assign", "bob")
			reply := auditRespond(t, c, root.ID, "y")
			args := auditExpand(tt.args, strings.NewReplacer("{root}", root.ID, "{reply}", reply.ID))

			bob := c
			bob.Author = "bob"
			if tt.wantErr != nil {
				if err := runErr(t, bob, "audit", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, bob, "audit", nil, args...)
			if got, want := text(resp), "Resolved "+root.ID+" [approved]"; got != want {
				t.Errorf("audit %v = %q, want %q", args, got, want)
			}
			aud := auditOf(t, resp)
			auditCompare(t, args, aud, sdk.Audit{ParentID: root.ID, Status: "approved", Author: "bob", Assignee: "bob"})
			if aud.Content != "" {
				t.Errorf("audit %v Content = %q, want empty", args, aud.Content)
			}
			if pending := auditsOf(t, run(t, c, "audit", nil, "list", "--pending")); len(pending) != 0 {
				t.Errorf("audit list --pending after resolve = %v, want none", pending)
			}
			if got := text(run(t, bob, "audit", nil, "status")); !strings.Contains(got, "No pending audits for bob") {
				t.Errorf("audit status after resolve = %q, want no pending audits", got)
			}
		})
	}
}

// TestAuditRmRestore ports the smoke scenario: rm hides the audit from
// show and list, restore brings it back, and the error cases for both.
func TestAuditRmRestore(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		deleted  bool // fixture audit is soft-deleted before the call
		wantText string
		wantErr  error
	}{
		{name: "rm hides the audit", args: []string{"rm", "{id}"}, wantText: "Deleted audit {id}"},
		{name: "rm unknown id", args: []string{"rm", "zzzzzzzzzz"}, wantErr: sdk.ErrNotFound},
		{name: "rm already deleted", args: []string{"rm", "{id}"}, deleted: true, wantErr: sdk.ErrNotFound},
		{name: "rm missing id", args: []string{"rm"}, wantErr: sdk.ErrMissingArg},
		{name: "restore recovers the audit", args: []string{"restore", "{id}"}, deleted: true, wantText: "Restored audit {id}"},
		{name: "restore a live audit", args: []string{"restore", "{id}"}, wantErr: sdk.ErrNotFound},
		{name: "restore unknown id", args: []string{"restore", "zzzzzzzzzz"}, wantErr: sdk.ErrNotFound},
		{name: "restore missing id", args: []string{"restore"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "smoke")
			id := auditCreate(t, c, "docs/greeting", "Needs review.").ID
			if tt.deleted {
				run(t, c, "audit", nil, "rm", id)
			}
			repl := strings.NewReplacer("{id}", id)
			args := auditExpand(tt.args, repl)

			if tt.wantErr != nil {
				if err := runErr(t, c, "audit", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, c, "audit", nil, args...)
			if got, want := text(resp), repl.Replace(tt.wantText); got != want {
				t.Errorf("audit %v = %q, want %q", args, got, want)
			}
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("audit %v response type = %T, want sdk.Text", args, resp)
			}

			err := runErr(t, c, "audit", nil, "show", id)
			listed := strings.Contains(text(run(t, c, "audit", nil, "list")), id)
			if args[0] == "rm" {
				if !errors.Is(err, sdk.ErrNotFound) {
					t.Errorf("audit show after rm error = %v, want %v", err, sdk.ErrNotFound)
				}
				if listed {
					t.Errorf("audit list after rm still contains %s", id)
				}
			} else {
				if err != nil {
					t.Errorf("audit show after restore error = %v", err)
				}
				if !strings.Contains(text(run(t, c, "audit", nil, "show", id)), "Needs review") {
					t.Errorf("audit show after restore does not contain the content")
				}
				if !listed {
					t.Errorf("audit list after restore does not contain %s", id)
				}
			}
		})
	}
}

// TestAuditStatus covers the inbox: threads whose effective assignee is
// the author and whose effective status is pending or needs-work.
// Fixture: {p} pending assigned to bob, {nw} needs-work assigned to
// bob after a reply, {other} pending assigned to carol, {done}
// assigned to bob then resolved, {moved} assigned to bob then
// reassigned to carol by a reply.
func TestAuditStatus(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		args       []string
		wantText   []string
		wantIDs    []string
		wantAbsent []string
		wantTotal  int
		wantErr    error
	}{
		{
			name:       "bob's inbox",
			author:     "bob",
			args:       []string{"status"},
			wantText:   []string{"Pending audits for bob: 2 (1 needs-work)"},
			wantIDs:    []string{"{p}", "{nw}"},
			wantAbsent: []string{"{other}", "{done}", "{moved}"},
			wantTotal:  2,
		},
		{
			name:       "carol's inbox includes the reassigned thread",
			author:     "carol",
			args:       []string{"status"},
			wantText:   []string{"Pending audits for carol: 2"},
			wantIDs:    []string{"{other}", "{moved}"},
			wantAbsent: []string{"{p}", "{nw}", "{done}", "needs-work)"},
			wantTotal:  2,
		},
		{
			name:     "empty inbox",
			author:   "dave",
			args:     []string{"status"},
			wantText: []string{"No pending audits for dave."},
		},
		{
			name:      "since a duration",
			author:    "bob",
			args:      []string{"status", "--since", "1h"},
			wantIDs:   []string{"{p}", "{nw}"},
			wantTotal: 2,
		},
		{
			name:     "since a future timestamp",
			author:   "bob",
			args:     []string{"status", "--since", "2100-01-01T00:00:00Z"},
			wantText: []string{"No pending audits for bob."},
		},
		{
			name:    "invalid since",
			author:  "bob",
			args:    []string{"status", "--since", "soon"},
			wantErr: errAuditAny,
		},
		{
			name:    "unknown flag",
			author:  "bob",
			args:    []string{"status", "--pending"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			p := auditCreate(t, c, "docs/a", "pending one", "--assign", "bob")
			nw := auditCreate(t, c, "docs/b", "needs work one", "--assign", "bob")
			auditRespond(t, c, nw.ID, "still broken", "--status", "needs-work")
			other := auditCreate(t, c, "docs/c", "for carol", "--assign", "carol")
			done := auditCreate(t, c, "docs/d", "done one", "--assign", "bob")
			run(t, c, "audit", nil, "resolve", done.ID)
			moved := auditCreate(t, c, "docs/e", "moved one", "--assign", "bob")
			auditRespond(t, c, moved.ID, "over to you", "--assign", "carol")
			repl := strings.NewReplacer("{p}", p.ID, "{nw}", nw.ID, "{other}", other.ID, "{done}", done.ID, "{moved}", moved.ID)
			args := auditExpand(tt.args, repl)

			me := c
			me.Author = tt.author
			if tt.wantErr != nil {
				err := runErr(t, me, "audit", nil, args...)
				if tt.wantErr == errAuditAny && err == nil {
					t.Errorf("audit %v error = nil, want an error", args)
				} else if tt.wantErr != errAuditAny && !errors.Is(err, tt.wantErr) {
					t.Errorf("audit %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, me, "audit", nil, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, ph := range tt.wantIDs {
				if id := repl.Replace(ph); !strings.Contains(got, id) {
					t.Errorf("audit %v = %q, want to contain %q", args, got, id)
				}
			}
			for _, ph := range tt.wantAbsent {
				if s := repl.Replace(ph); strings.Contains(got, s) {
					t.Errorf("audit %v = %q, must not contain %q", args, got, s)
				}
			}
			st, ok := data(resp).(*sdk.AuditStatus)
			if !ok || st == nil {
				t.Fatalf("audit %v Data = %T, want *sdk.AuditStatus", args, data(resp))
			}
			if st.Author != tt.author {
				t.Errorf("audit %v Data.Author = %q, want %q", args, st.Author, tt.author)
			}
			if st.Summary.Total != tt.wantTotal || len(st.Pending) != tt.wantTotal {
				t.Errorf("audit %v Summary.Total = %d, Pending = %d, want %d", args, st.Summary.Total, len(st.Pending), tt.wantTotal)
			}
			if st.Summary.Pending+st.Summary.NeedsWork != st.Summary.Total {
				t.Errorf("audit %v Summary = %+v, want pending + needs-work == total", args, st.Summary)
			}
		})
	}
}
