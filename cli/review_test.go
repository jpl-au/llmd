package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// metaAddTask creates a task through the dispatcher and returns it.
func metaAddTask(t *testing.T, c sdk.Context, spec string, args ...string) *sdk.Task {
	t.Helper()
	var stdin []byte
	if spec != "" {
		stdin = []byte(spec)
	}
	resp := run(t, c, "task", stdin, append([]string{"add"}, args...)...)
	task, ok := data(resp).(*sdk.Task)
	if !ok {
		t.Fatalf("task add Data = %T, want *sdk.Task", data(resp))
	}
	return task
}

// TestReview covers both renderings: the plain tab-separated listing
// when stdout is not a TTY (the default under go test) and the styled
// cards when it is. It checks the column filter as flag and
// positional, the -n limit, the spec preview and links in the data
// payload, the empty board, and flag errors. Each case runs against a
// fresh store with two backlog tasks (one with a spec linked to a
// document) and one task in progress.
func TestReview(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		empty      bool
		tty        bool // render for a terminal
		args       []string
		wantText   []string
		wantAbsent []string
		wantCount  int      // entries in the data payload
		wantJSON   []string // substrings of the JSON-encoded payload
	}{
		{
			name:   "tty renders cards",
			author: "alice",
			tty:    true,
			args:   nil,
			wantText: []string{
				"Fix login", "BACKLOG", "Priority: 2", "Assigned: bob",
				"Spec:", "Users cannot log in.", "See the error log.",
				"Links:", "docs/ref (context)",
				"Write docs", "Priority: 0", "Deploy", "IN-PROGRESS", "3 tasks",
			},
			wantAbsent: []string{"\t", "no tasks", "Flags:"},
			wantCount:  3,
		},
		{
			name:       "tty column filter",
			author:     "alice",
			tty:        true,
			args:       []string{"in-progress"},
			wantText:   []string{"Deploy", "IN-PROGRESS", "1 tasks"},
			wantAbsent: []string{"Fix login", "Spec:", "Links:"},
			wantCount:  1,
		},
		{
			name:       "tty empty board",
			author:     "alice",
			empty:      true,
			tty:        true,
			args:       nil,
			wantText:   []string{"no tasks"},
			wantAbsent: []string{"tasks\n", "Priority"},
			wantCount:  0,
		},
		{
			name:      "all tasks",
			author:    "alice",
			args:      nil,
			wantText:  []string{"\tFix login\tbacklog\tP2\tbob\t\ttasks/fix-login\n", "\tWrite docs\tbacklog\tP0\t\t\t\n", "\tDeploy\tin-progress\tP0\t\t\t\n"},
			wantCount: 3,
			wantJSON:  []string{`"task":{`, `"spec":"Users cannot log in.\nSee the error log.\n"`, `"links":[{`, `"To":"docs/ref"`, `"Label":"context"`},
		},
		{
			name:       "column flag",
			author:     "alice",
			args:       []string{"--column", "in-progress"},
			wantText:   []string{"\tDeploy\tin-progress\t"},
			wantAbsent: []string{"Fix login", "Write docs"},
			wantCount:  1,
		},
		{
			name:       "column positional",
			author:     "alice",
			args:       []string{"backlog"},
			wantText:   []string{"Fix login", "Write docs"},
			wantAbsent: []string{"Deploy"},
			wantCount:  2,
		},
		{
			name:       "flag wins over positional",
			author:     "alice",
			args:       []string{"--column", "in-progress", "backlog"},
			wantText:   []string{"Deploy"},
			wantAbsent: []string{"Fix login"},
			wantCount:  1,
		},
		{
			name:      "limit",
			author:    "alice",
			args:      []string{"-n", "1"},
			wantCount: 1,
		},
		{
			name:      "limit larger than the board",
			author:    "alice",
			args:      []string{"-n", "10"},
			wantCount: 3,
		},
		{
			name:       "unknown column is empty",
			author:     "alice",
			args:       []string{"--column", "nope"},
			wantAbsent: []string{"\t"},
			wantCount:  0,
		},
		{
			name:       "empty board",
			author:     "alice",
			empty:      true,
			args:       nil,
			wantAbsent: []string{"\t"},
			wantCount:  0,
		},
		{
			name:      "without author",
			author:    "",
			args:      nil,
			wantText:  []string{"Fix login"},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := newCLI(t, "alice")
			if !tt.empty {
				writeDoc(t, seed, "docs/ref", "# Ref")
				login := metaAddTask(t, seed, "# Fix login\n\nUsers cannot log in.\nSee the error log.\n", "Fix login", "--priority", "2", "--assign", "bob")
				if login.Path == "" {
					t.Fatal("task with a spec has no path")
				}
				run(t, seed, "link", nil, "--label", "context", login.Path, "docs/ref")
				metaAddTask(t, seed, "", "Write docs")
				metaAddTask(t, seed, "", "Deploy", "--column", "in-progress")
			}
			c := seed
			c.Author = tt.author

			var resp sdk.Response
			if tt.tty {
				metaWithTTY(t, func() { resp = run(t, c, "review", nil, tt.args...) })
			} else {
				resp = run(t, c, "review", nil, tt.args...)
			}
			got := text(resp)
			if tt.tty {
				got = metaStripANSI(got)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("review %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("review %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if !tt.tty {
				if strings.ContainsAny(got, "\x1b─") {
					t.Errorf("review %v = %q, want plain text when stdout is not a TTY", tt.args, got)
				}
				if lines := strings.Count(got, "\n"); lines != tt.wantCount {
					t.Errorf("review %v = %d lines, want %d", tt.args, lines, tt.wantCount)
				}
				// Each plain line is the seven-field task line, starting
				// with a 9-character task key.
				for line := range strings.SplitSeq(strings.TrimRight(got, "\n"), "\n") {
					if line == "" {
						continue
					}
					if fields := strings.Split(line, "\t"); len(fields) != 7 || len(fields[0]) != 9 {
						t.Errorf("review %v line %q, want key\\ttitle\\tcolumn\\tP<n>\\tassignee\\tflags\\tspec", tt.args, line)
					}
				}
			} else if tt.wantCount > 1 && strings.Count(got, "────") < tt.wantCount-1 {
				t.Errorf("review %v = %q, want %d separators between cards", tt.args, got, tt.wantCount-1)
			}
			raw, err := json.Marshal(data(resp))
			if err != nil {
				t.Fatalf("review %v Data does not encode: %v", tt.args, err)
			}
			var entries []map[string]any
			if err := json.Unmarshal(raw, &entries); err != nil {
				t.Fatalf("review %v Data = %s, want a JSON array of entries", tt.args, raw)
			}
			if len(entries) != tt.wantCount {
				t.Errorf("review %v Data = %d entries, want %d", tt.args, len(entries), tt.wantCount)
			}
			for _, want := range tt.wantJSON {
				if !strings.Contains(string(raw), want) {
					t.Errorf("review %v Data = %s, want to contain %s", tt.args, raw, want)
				}
			}
		})
	}
}

// TestReviewErrors covers flag errors.
func TestReviewErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "unknown flag", args: []string{"--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "limit is not an integer", args: []string{"-n", "lots"}, wantErr: sdk.ErrInvalidArg},
		{name: "column without value", args: []string{"--column"}, wantErr: sdk.ErrInvalidArg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			if err := runErr(t, c, "review", nil, tt.args...); !errors.Is(err, tt.wantErr) {
				t.Errorf("review %v error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

// TestReviewPending checks what review leaves out by default: done
// tasks and tasks flagged human, with a note counting them. --all and
// an explicit column show everything. Each case starts with a task in
// progress, one done, and one in up-next flagged human.
func TestReviewPending(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantText   []string
		wantAbsent []string
		wantCount  int
	}{
		{
			name:       "done and human are left out with a note",
			args:       nil,
			wantText:   []string{"Deploy", "(1 done, 1 human not shown; use --all)"},
			wantAbsent: []string{"Shipped", "Decide"},
			wantCount:  1,
		},
		{
			name:       "--all shows everything",
			args:       []string{"--all"},
			wantText:   []string{"Deploy", "Shipped", "Decide"},
			wantAbsent: []string{"not shown"},
			wantCount:  3,
		},
		{
			name:       "an explicit column is shown in full",
			args:       []string{"--column", "done"},
			wantText:   []string{"Shipped"},
			wantAbsent: []string{"not shown", "Deploy"},
			wantCount:  1,
		},
		{
			name:       "a human task in the asked-for column is shown",
			args:       []string{"up-next"},
			wantText:   []string{"Decide", "human"},
			wantAbsent: []string{"not shown"},
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			metaAddTask(t, c, "# Deploy\n\nShip it.", "Deploy", "--column", "in-progress")
			metaAddTask(t, c, "# Shipped\n\nDone.", "Shipped", "--column", "done")
			decide := metaAddTask(t, c, "# Decide\n\nYour call.", "Decide", "--column", "up-next")
			run(t, c, "task", nil, "set", decide.Key, "--flag", "human")

			resp := run(t, c, "review", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("review %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("review %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			b, err := json.Marshal(data(resp))
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(b), `"task":{`); n != tt.wantCount {
				t.Errorf("review %v payload has %d entries, want %d", tt.args, n, tt.wantCount)
			}
		})
	}
}
