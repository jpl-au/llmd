package cli

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// runTTY runs cmd as the CLI does for a person at a terminal:
// sdk.Context.TTY is set, so commands take their terminal-rendering
// branch. (Dispatch, which run uses, is MCP and HTTP: never a terminal.)
func runTTY(t *testing.T, c sdk.Context, cmd string, args ...string) sdk.Response {
	t.Helper()
	c.TTY = true
	resp, err := (&CLI{}).Exec(c, cmd, args)
	if err != nil {
		t.Fatalf("%s %v: %v", cmd, args, err)
	}
	return resp
}

// metaANSI matches SGR escape sequences emitted by lipgloss.
var metaANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// metaStripANSI removes colour and style escape sequences.
func metaStripANSI(s string) string { return metaANSI.ReplaceAllString(s, "") }

// TestStatus covers the dashboard in both renderings: plain text when
// stdout is not a TTY (the default under go test) and lipgloss tables
// when it is. It checks the recent documents section newest first,
// the -n limit, the task board counts, the activity feed, the empty
// store, and flag errors. Each case runs against a fresh store with
// three documents written at increasing times and one task.
func TestStatus(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		empty      bool // skip the fixture
		tty        bool // render for a terminal
		args       []string
		wantText   []string
		wantAbsent []string
		wantDocs   int // entries under recent_documents
		wantErr    error
	}{
		{
			name:   "tty renders tables",
			author: "alice",
			tty:    true,
			args:   nil,
			wantText: []string{
				"RECENT DOCUMENTS", "PATH", "VER", "AUTHOR", "DATE", "docs/c", "docs/b", "docs/a", "bob",
				"TASK BOARD", "1", "backlog", "done",
				"RECENT ACTIVITY", "TIME", "TYPE", "SUBJECT", "ACTION", "DETAIL", "✨ written", "Ship it",
			},
			wantAbsent: []string{"Recent documents:", "no documents", "no activity"},
			wantDocs:   3,
		},
		{
			name:       "tty limit",
			author:     "alice",
			tty:        true,
			args:       []string{"-n", "1"},
			wantText:   []string{"docs/c"},
			wantAbsent: []string{"docs/b", "docs/a"},
			wantDocs:   1,
		},
		{
			name:       "tty empty store",
			author:     "alice",
			empty:      true,
			tty:        true,
			args:       nil,
			wantText:   []string{"RECENT DOCUMENTS", "no documents", "TASK BOARD", "0", "backlog", "RECENT ACTIVITY", "no activity"},
			wantAbsent: []string{"PATH", "TIME"},
		},
		{
			name:   "default sections",
			author: "alice",
			args:   nil,
			wantText: []string{
				"Recent documents:\n", "  docs/c v1 alice\n", "  docs/b v2 bob\n", "  docs/a v1 alice\n",
				"Task board:\n", "  backlog: 1\n", "  done: 0\n",
				"Recent activity:\n", " task ", " document docs/c written",
			},
			wantDocs: 3,
		},
		{
			name:       "limit caps each section",
			author:     "alice",
			args:       []string{"--limit", "2"},
			wantText:   []string{"  docs/c v1 alice\n", "  docs/b v2 bob\n"},
			wantAbsent: []string{"docs/a"},
			wantDocs:   2,
		},
		{
			name:       "limit of one",
			author:     "alice",
			args:       []string{"-n", "1"},
			wantText:   []string{"  docs/c v1 alice\n"},
			wantAbsent: []string{"docs/b", "docs/a"},
			wantDocs:   1,
		},
		{
			name:     "newest first",
			author:   "alice",
			args:     nil,
			wantText: []string{"docs/c v1 alice\n  docs/b v2 bob\n  docs/a v1 alice"},
			wantDocs: 3,
		},
		{
			name:       "empty store",
			author:     "alice",
			empty:      true,
			args:       nil,
			wantText:   []string{"Recent documents:\n", "Task board:\n", "  backlog: 0\n", "Recent activity:\n"},
			wantAbsent: []string{"docs/"},
		},
		{
			name:     "without author",
			author:   "",
			args:     nil,
			wantText: []string{"  docs/c v1 alice\n", "  backlog: 1\n"},
			wantDocs: 3,
		},
		{
			name:    "unknown flag",
			author:  "alice",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "limit is not an integer",
			author:  "alice",
			args:    []string{"-n", "lots"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clocktest.Sequence(t, 1_700_000_000_000, 1000)
			seed := newCLI(t, "alice")
			if !tt.empty {
				writeDoc(t, seed, "docs/a", "# A")
				writeDoc(t, seed, "docs/b", "# B")
				bob := seed
				bob.Author = "bob"
				writeDoc(t, bob, "docs/b", "# B v2")
				writeDoc(t, seed, "docs/c", "# C")
				run(t, seed, "task", nil, "add", "Ship it")
			}
			c := seed
			c.Author = tt.author

			if tt.wantErr != nil {
				if err := runErr(t, c, "status", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("status %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			var resp sdk.Response
			if tt.tty {
				resp = runTTY(t, c, "status", tt.args...)
			} else {
				resp = run(t, c, "status", nil, tt.args...)
			}
			got := text(resp)
			if tt.tty {
				got = metaStripANSI(got)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("status %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("status %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if !tt.tty && strings.ContainsAny(got, "\x1b│╭") {
				t.Errorf("status %v = %q, want plain text when stdout is not a TTY", tt.args, got)
			}
			if tt.tty && !tt.empty && !strings.ContainsAny(got, "│╭") {
				t.Errorf("status %v = %q, want table borders when stdout is a TTY", tt.args, got)
			}
			d, ok := data(resp).(map[string]any)
			if !ok {
				t.Fatalf("status %v Data = %T, want map[string]any", tt.args, data(resp))
			}
			for _, k := range []string{"recent_documents", "task_summary", "recent_activity"} {
				if _, ok := d[k]; !ok {
					t.Errorf("status %v Data missing %q: %v", tt.args, k, d)
				}
			}
			docs, _ := d["recent_documents"].([]sdk.Doc)
			if len(docs) != tt.wantDocs {
				t.Errorf("status %v recent_documents = %d entries, want %d", tt.args, len(docs), tt.wantDocs)
			}
			if tt.wantDocs > 0 && docs[0].Path != "docs/c" {
				t.Errorf("status %v recent_documents[0] = %s, want docs/c", tt.args, docs[0].Path)
			}
			summary, _ := d["task_summary"].(map[string]int)
			if want := map[bool]int{true: 0, false: 1}[tt.empty]; summary["backlog"] != want {
				t.Errorf("status %v task_summary[backlog] = %d, want %d", tt.args, summary["backlog"], want)
			}
		})
	}
}
