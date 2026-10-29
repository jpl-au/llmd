package cli

import (
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// TestTaskLink covers linking a task's spec document to another
// document: the happy path, missing target, a task whose spec document
// was never written, duplicates, self-links and the argument errors.
func TestTaskLink(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		noBody      bool // seed Alpha without a spec document
		setup       func(t *testing.T, c sdk.Context, tk *sdk.Task)
		args        []string // {key} is Alpha, {path} is its spec path
		wantErr     error
		wantErrText string
	}{
		{name: "links to a document", args: []string{"link", "{key}", "docs/design"}},
		{
			name: "duplicate link",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				run(t, c, "task", nil, "link", tk.Key, "docs/design")
			},
			args:    []string{"link", "{key}", "docs/design"},
			wantErr: sdk.ErrExists,
		},
		{name: "self link", args: []string{"link", "{key}", "{path}"}, wantErr: sdk.ErrInvalidArg},
		{name: "missing target document", args: []string{"link", "{key}", "docs/missing"}, wantErr: sdk.ErrNotFound},
		{name: "task without a spec document", noBody: true, args: []string{"link", "{key}", "docs/design"}, wantErr: sdk.ErrNotFound},
		{name: "missing task", args: []string{"link", "nope", "docs/design"}, wantErr: sdk.ErrNotFound},
		{name: "missing path", args: []string{"link", "{key}"}, wantErr: sdk.ErrMissingArg},
		{name: "no args", args: []string{"link"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, args: []string{"link", "{key}", "docs/design"}, wantErrText: "an author is required."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/design", "# Design\n\nDetails.")
			body := taskBody("Alpha")
			if tt.noBody {
				body = nil
			}
			seed := taskSeed(t, c, "Alpha", body)
			if tt.setup != nil {
				tt.setup(t, c, seed)
			}
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, taskVars{"key": seed.Key, "path": seed.Path})

			if taskExpectErr(t, c, "task", args, tt.wantErr, tt.wantErrText) {
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("task %v response type = %T, want sdk.Text", args, resp)
			}
			taskWantText(t, "task", args, text(resp), []string{"Linked task " + seed.Key + " to docs/design"}, nil)

			links, err := c.Links.List(seed.Path, "out")
			if err != nil || len(links) != 1 || links[0].From != seed.Path || links[0].To != "docs/design" {
				t.Errorf("Links.List(%s) = %+v, %v, want one link to docs/design", seed.Path, links, err)
			}
		})
	}
}

// TestTaskLinks covers listing a task's outgoing links: none, plain,
// labelled, several, a task with no spec document, and the errors.
func TestTaskLinks(t *testing.T) {
	tests := []struct {
		name       string
		noAuthor   bool
		noBody     bool
		setup      func(t *testing.T, c sdk.Context, tk *sdk.Task)
		args       []string
		wantText   string
		wantTo     []string // Data link targets in order
		wantErr    error
		wantAbsent []string
	}{
		{name: "no links", args: []string{"links", "{key}"}, wantText: "", wantTo: []string{}},
		{
			name: "one link",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				run(t, c, "task", nil, "link", tk.Key, "docs/design")
			},
			args:     []string{"links", "{key}"},
			wantText: "docs/design",
			wantTo:   []string{"docs/design"},
		},
		{
			name: "labelled link",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				if err := c.Links.Add(tk.Path, "docs/design", "ref", "alice"); err != nil {
					t.Fatalf("Links.Add: %v", err)
				}
			},
			args:     []string{"links", "{key}"},
			wantText: "docs/design (ref)",
			wantTo:   []string{"docs/design"},
		},
		{
			// Links are listed newest first.
			name: "several links one per line",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				writeDoc(t, c, "docs/other", "# Other\n\nDetails.")
				run(t, c, "task", nil, "link", tk.Key, "docs/design")
				run(t, c, "task", nil, "link", tk.Key, "docs/other")
			},
			args:     []string{"links", "{key}"},
			wantText: "docs/other\ndocs/design",
			wantTo:   []string{"docs/other", "docs/design"},
		},
		{
			// Only outgoing links are listed.
			name: "incoming links are not listed",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				if err := c.Links.Add("docs/design", tk.Path, "", "alice"); err != nil {
					t.Fatalf("Links.Add: %v", err)
				}
			},
			args:       []string{"links", "{key}"},
			wantAbsent: []string{"docs/design"},
			wantTo:     []string{},
		},
		{name: "task without a spec document has none", noBody: true, args: []string{"links", "{key}"}, wantText: "", wantTo: []string{}},
		{
			name: "reads without author",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) {
				run(t, c, "task", nil, "link", tk.Key, "docs/design")
			},
			noAuthor: true,
			args:     []string{"links", "{key}"},
			wantText: "docs/design",
			wantTo:   []string{"docs/design"},
		},
		{name: "missing task", args: []string{"links", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", args: []string{"links"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Links list newest first by millisecond timestamp; a strictly
			// increasing clock keeps that order deterministic.
			clocktest.Sequence(t, 1_700_000_000_000, 1)
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/design", "# Design\n\nDetails.")
			body := taskBody("Alpha")
			if tt.noBody {
				body = nil
			}
			seed := taskSeed(t, c, "Alpha", body)
			if tt.setup != nil {
				tt.setup(t, c, seed)
			}
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, taskVars{"key": seed.Key})

			if tt.wantErr != nil {
				taskWantErr(t, "task", args, runErr(t, c, "task", nil, args...), tt.wantErr, "")
				return
			}
			resp := run(t, c, "task", nil, args...)
			if got := text(resp); got != tt.wantText && len(tt.wantAbsent) == 0 {
				t.Errorf("task %v = %q, want %q", args, got, tt.wantText)
			}
			taskWantText(t, "task", args, text(resp), nil, tt.wantAbsent)

			links, _ := data(resp).([]sdk.Link)
			got := make([]string, len(links))
			for i, l := range links {
				got[i] = l.To
			}
			if len(got) != len(tt.wantTo) {
				t.Fatalf("task %v Data targets = %v, want %v", args, got, tt.wantTo)
			}
			for i := range got {
				if got[i] != tt.wantTo[i] {
					t.Errorf("task %v Data targets = %v, want %v", args, got, tt.wantTo)
				}
			}
		})
	}
}
