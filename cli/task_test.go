package cli

// task_test.go holds the helpers shared by the task and rule command
// tests (all prefixed "task" so they cannot collide with other test
// groups), plus the tests for task dispatch and task list.

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/sdk"
)

// taskVars maps {placeholder} names to values produced by a case's
// setup (task keys, temp file paths) for substitution into args.
type taskVars = map[string]string

// taskBody returns a spec body that passes spec gating: a title
// heading followed by real content.
func taskBody(title string) []byte {
	return []byte("# " + title + "\n\n" + title + " spec body.")
}

// taskSeed creates a task through the CLI, so dispatch, flag parsing
// and the add path are exercised, and returns the stored task. body is
// piped on stdin; nil creates a spec-less task.
func taskSeed(t *testing.T, c sdk.Context, title string, body []byte, args ...string) *sdk.Task {
	t.Helper()
	resp := run(t, c, "task", body, append([]string{"add", title}, args...)...)
	tk, ok := data(resp).(*sdk.Task)
	if !ok || tk == nil {
		t.Fatalf("task add %q Data = %T, want *sdk.Task", title, data(resp))
	}
	return tk
}

// taskWorkdirCLI is newCLI with the working directory set to the store
// directory. Column add/rm/mv and every rule subcommand write
// .llmd/rules and .llmd/.gitignore relative to the cwd, so those tests
// use this to keep the files inside the temp store.
func taskWorkdirCLI(t *testing.T, author string) sdk.Context {
	t.Helper()
	return host.TestSetup(t, host.TestWorkdir).Context(t.Context(), author)
}

// taskRead returns the stored task or fails the test.
func taskRead(t *testing.T, c sdk.Context, key string) *sdk.Task {
	t.Helper()
	tk, err := c.Tasks.Read(key)
	if err != nil {
		t.Fatalf("Tasks.Read(%s): %v", key, err)
	}
	return tk
}

// taskExpand substitutes {name} placeholders in args from vars.
func taskExpand(args []string, vars taskVars) []string {
	out := make([]string, len(args))
	pairs := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	r := strings.NewReplacer(pairs...)
	for i, a := range args {
		out[i] = r.Replace(a)
	}
	return out
}

// taskTitles returns the titles of tasks in order.
func taskTitles(tasks []*sdk.Task) []string {
	out := make([]string, len(tasks))
	for i, tk := range tasks {
		out[i] = tk.Title
	}
	return out
}

// taskWantErr asserts err against the sentinel want, or against the
// substring wantText when no sentinel applies, and reports whether
// the case expected an error at all.
func taskWantErr(t *testing.T, cmd string, args []string, err, want error, wantText string) bool {
	t.Helper()
	switch {
	case want != nil:
		if !errors.Is(err, want) {
			t.Errorf("%s %v error = %v, want %v", cmd, args, err, want)
		}
		return true
	case wantText != "":
		if err == nil || !strings.Contains(err.Error(), wantText) {
			t.Errorf("%s %v error = %v, want to contain %q", cmd, args, err, wantText)
		}
		return true
	}
	return false
}

// taskExpectErr reports whether the case expects an error and, when it
// does, runs cmd and asserts the error. The command is not run at all
// for success cases, so the caller's own run is the only execution.
func taskExpectErr(t *testing.T, c sdk.Context, cmd string, args []string, want error, wantText string) bool {
	t.Helper()
	if want == nil && wantText == "" {
		return false
	}
	return taskWantErr(t, cmd, args, runErr(t, c, cmd, nil, args...), want, wantText)
}

// taskWantText asserts the substrings that must and must not appear
// in a command's human-readable output.
func taskWantText(t *testing.T, cmd string, args []string, got string, want, absent []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("%s %v = %q, want to contain %q", cmd, args, got, w)
		}
	}
	for _, a := range absent {
		if strings.Contains(got, a) {
			t.Errorf("%s %v = %q, must not contain %q", cmd, args, got, a)
		}
	}
}

// TestTaskDispatch covers the subcommand switch: missing and unknown
// subcommands, and the mixed-command author rule - reads work with no
// author, mutations are refused without one.
func TestTaskDispatch(t *testing.T) {
	tests := []struct {
		name        string
		author      string
		args        []string // {key} is a seeded backlog task
		wantErr     error
		wantErrText string
		skip        string
	}{
		{name: "no subcommand", author: "alice", wantErr: sdk.ErrMissingArg},
		{name: "unknown subcommand", author: "alice", args: []string{"bogus"}, wantErrText: "unknown subcommand"},
		{name: "unknown column subcommand", author: "alice", args: []string{"column", "bogus"}, wantErrText: "unknown subcommand"},

		// Reads need no author.
		{name: "list reads without author", args: []string{"list"}},
		{name: "show reads without author", args: []string{"show", "{key}"}},
		{name: "log reads without author", args: []string{"log", "{key}"}},
		{name: "chain reads without author", args: []string{"chain", "{key}"}},
		{name: "ready reads without author", args: []string{"ready", "{key}"}},
		{name: "links reads without author", args: []string{"links", "{key}"}},
		{name: "column list reads without author", args: []string{"column", "list"}},
		{name: "bare column lists without author", args: []string{"column"}},

		// Mutations are refused without an author.
		{name: "add needs author", args: []string{"add", "Title"}, wantErrText: "an author is required."},
		{name: "move needs author", args: []string{"move", "{key}", "backlog"}, wantErrText: "an author is required."},
		{name: "set needs author", args: []string{"set", "{key}", "--priority", "1"}, wantErrText: "an author is required."},
		{name: "rm needs author", args: []string{"rm", "{key}"}, wantErrText: "an author is required."},
		{name: "restore needs author", args: []string{"restore", "{key}"}, wantErrText: "an author is required."},
		{name: "link needs author", args: []string{"link", "{key}", "docs/x"}, wantErrText: "an author is required."},
		{name: "column add needs author", args: []string{"column", "add", "testing"}, wantErrText: "an author is required."},
		{name: "column rm needs author", args: []string{"column", "rm", "approval"}, wantErrText: "an author is required."},
		{name: "column mv needs author", args: []string{"column", "mv", "approval", "--after", "backlog"}, wantErrText: "an author is required."},
		{name: "column move alias needs author", args: []string{"column", "move", "approval", "--after", "backlog"}, wantErrText: "an author is required."},
		{
			name:    "mutation without author is ErrMissingArg",
			args:    []string{"add", "Title"},
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			h := host.TestSetup(t, host.TestIsolated)
			seed := taskSeed(t, h.Context(t.Context(), "alice"), "Seeded", taskBody("Seeded"))
			c := h.Context(t.Context(), tt.author)
			args := taskExpand(tt.args, taskVars{"key": seed.Key})

			err := runErr(t, c, "task", nil, args...)
			if taskWantErr(t, "task", args, err, tt.wantErr, tt.wantErrText) {
				return
			}
			if err != nil {
				t.Errorf("task %v error = %v, want success", args, err)
			}
		})
	}
}

// TestTaskList covers the board view, the flat column table (positional
// and --column), the assignee and priority filters, aliases and the
// flag errors. Each case runs against a fresh board holding Alpha
// (backlog, spec, priority 1, alice), Beta (backlog, no spec, priority
// 2) and Gamma (in-progress, spec, bob, flagged blocked).
func TestTaskList(t *testing.T) {
	tests := []struct {
		name       string
		noAuthor   bool
		args       []string
		wantText   []string
		wantAbsent []string
		wantData   []string // titles in Data order; nil skips the check
		wantErr    error
	}{
		{
			name:     "board view groups every column",
			args:     []string{"list"},
			wantText: []string{"BACKLOG (2)", "UP-NEXT (0)", "IN-PROGRESS (1)", "REVIEW (0)", "APPROVAL (0)", "DONE (0)", "BLOCKED (0)", "Alpha", "Beta", "Gamma"},
			wantData: []string{"Alpha", "Beta", "Gamma"},
		},
		{
			// The SPEC column shows the path only when the document exists;
			// FLAGS shows the task's flags.
			name:       "board view shows specs and flags",
			args:       []string{"list"},
			wantText:   []string{"tasks/alpha", "tasks/gamma", "blocked", "bob"},
			wantAbsent: []string{"tasks/beta"},
		},
		{name: "ls alias", args: []string{"ls"}, wantText: []string{"Alpha"}, wantData: []string{"Alpha", "Beta", "Gamma"}},
		{name: "board alias", args: []string{"board"}, wantText: []string{"Gamma"}, wantData: []string{"Alpha", "Beta", "Gamma"}},
		{
			// Piped, a column is one seven-field task line per task.
			name:       "positional column is a flat list",
			args:       []string{"list", "backlog"},
			wantText:   []string{"\tAlpha\tbacklog\tP1\talice\t\ttasks/alpha\n", "\tBeta\tbacklog\tP2\t\t\t\n"},
			wantAbsent: []string{"Gamma", "BACKLOG", "IN-PROGRESS", "│"},
			wantData:   []string{"Alpha", "Beta"},
		},
		{
			name:       "--column filters",
			args:       []string{"list", "--column", "in-progress"},
			wantText:   []string{"Gamma"},
			wantAbsent: []string{"Alpha", "Beta"},
			wantData:   []string{"Gamma"},
		},
		{
			name:       "--column wins over the positional column",
			args:       []string{"list", "--column", "in-progress", "backlog"},
			wantText:   []string{"Gamma"},
			wantAbsent: []string{"Alpha"},
			wantData:   []string{"Gamma"},
		},
		{
			name:       "--assign filters the board",
			args:       []string{"list", "--assign", "bob"},
			wantText:   []string{"Gamma", "IN-PROGRESS", "BACKLOG"},
			wantAbsent: []string{"Alpha", "Beta"},
			wantData:   []string{"Gamma"},
		},
		{
			name:       "--priority filters",
			args:       []string{"list", "--priority", "2"},
			wantText:   []string{"Beta"},
			wantAbsent: []string{"Alpha", "Gamma"},
			wantData:   []string{"Beta"},
		},
		{
			name:       "empty column",
			args:       []string{"list", "done"},
			wantText:   []string{"no tasks"},
			wantAbsent: []string{"Alpha", "Beta", "Gamma"},
			wantData:   []string{},
		},
		{
			name:     "unknown column filter is simply empty",
			args:     []string{"list", "nope"},
			wantText: []string{"no tasks"},
			wantData: []string{},
		},
		{name: "reads without author", noAuthor: true, args: []string{"list"}, wantData: []string{"Alpha", "Beta", "Gamma"}},
		{name: "unknown flag", args: []string{"list", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "--priority not an integer", args: []string{"list", "--priority", "high"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := host.TestSetup(t, host.TestIsolated)
			seed := h.Context(t.Context(), "alice")
			taskSeed(t, seed, "Alpha", taskBody("Alpha"), "--priority", "1", "--assign", "alice")
			taskSeed(t, seed, "Beta", nil, "--priority", "2")
			gamma := taskSeed(t, seed, "Gamma", taskBody("Gamma"), "--column", "in-progress", "--assign", "bob")
			run(t, seed, "task", nil, "set", gamma.Key, "--flag", "blocked")

			author := "alice"
			if tt.noAuthor {
				author = ""
			}
			c := h.Context(t.Context(), author)

			if tt.wantErr != nil {
				if err := runErr(t, c, "task", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("task %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "task", nil, tt.args...)
			taskWantText(t, "task", tt.args, text(resp), tt.wantText, tt.wantAbsent)
			if tt.wantData != nil {
				tasks, ok := data(resp).([]*sdk.Task)
				if !ok {
					t.Fatalf("task %v Data = %T, want []*sdk.Task", tt.args, data(resp))
				}
				if got := taskTitles(tasks); !slices.Equal(got, tt.wantData) {
					t.Errorf("task %v Data titles = %v, want %v", tt.args, got, tt.wantData)
				}
			}
		})
	}
}

// TestTaskListSince drives --since with a deterministic clock. Tasks
// are stamped from a fake epoch in whole seconds so an RFC 3339 cutoff
// (second precision) round-trips exactly; the strict "created after"
// comparison excludes a task created at the cutoff itself. Durations
// are relative to the real clock, so "1h" excludes the fake epoch.
func TestTaskListSince(t *testing.T) {
	const start = int64(1_700_000_000_000) // 2023-11-14T22:13:20Z
	rfc := func(tk *sdk.Task) string { return time.UnixMilli(tk.CreatedAt).UTC().Format(time.RFC3339) }

	tests := []struct {
		name        string
		since       func(first, second *sdk.Task) string
		wantText    []string
		wantAbsent  []string
		wantData    []string // titles in Data order
		wantErrText string
	}{
		{
			name:       "cutoff at the first task excludes it",
			since:      func(first, _ *sdk.Task) string { return rfc(first) },
			wantText:   []string{"Second"},
			wantAbsent: []string{"First"},
			wantData:   []string{"Second"},
		},
		{
			name:     "cutoff before both includes both",
			since:    func(_, _ *sdk.Task) string { return "2000-01-01T00:00:00Z" },
			wantText: []string{"First", "Second"},
			wantData: []string{"First", "Second"},
		},
		{
			name:       "cutoff at the second task excludes both",
			since:      func(_, second *sdk.Task) string { return rfc(second) },
			wantText:   []string{"BACKLOG (0)"},
			wantAbsent: []string{"First", "Second"},
			wantData:   []string{},
		},
		{
			name:       "duration is relative to the real clock",
			since:      func(_, _ *sdk.Task) string { return "1h" },
			wantAbsent: []string{"First", "Second"},
			wantData:   []string{},
		},
		{
			name:       "whole days are accepted",
			since:      func(_, _ *sdk.Task) string { return "7d" },
			wantAbsent: []string{"First", "Second"},
			wantData:   []string{},
		},
		{
			name:        "invalid value",
			since:       func(_, _ *sdk.Task) string { return "yesterday" },
			wantErrText: "yesterday is not a time.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clocktest.Sequence(t, start, 1000)
			c := newCLI(t, "alice")
			first := taskSeed(t, c, "First", nil)
			second := taskSeed(t, c, "Second", nil)
			if first.CreatedAt%1000 != 0 || second.CreatedAt <= first.CreatedAt {
				t.Fatalf("fake clock: first=%d second=%d, want increasing whole seconds", first.CreatedAt, second.CreatedAt)
			}

			args := []string{"list", "--since", tt.since(first, second)}
			if tt.wantErrText != "" {
				err := runErr(t, c, "task", nil, args...)
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("task %v error = %v, want to contain %q", args, err, tt.wantErrText)
				}
				return
			}
			resp := run(t, c, "task", nil, args...)
			taskWantText(t, "task", args, text(resp), tt.wantText, tt.wantAbsent)

			tasks, ok := data(resp).([]*sdk.Task)
			if !ok {
				t.Fatalf("task %v Data = %T, want []*sdk.Task", args, data(resp))
			}
			if got := taskTitles(tasks); !slices.Equal(got, tt.wantData) {
				t.Errorf("task %v Data titles = %v, want %v", args, got, tt.wantData)
			}
		})
	}
}

// TestTaskListOrder checks the order within a column as the board and
// task next present it: by default a task with priority 1 comes before
// priority 3, which comes before no priority, and board position only
// breaks ties; a host configured for position order shows the column
// as created.
func TestTaskListOrder(t *testing.T) {
	tests := []struct {
		name string
		opts []host.Option
		want []string
	}{
		{"priority first by default", nil, []string{"High", "Low", "Low too", "None"}},
		{"position when configured", []host.Option{host.WithTaskOrder(tasks.OrderPosition)}, []string{"None", "Low", "High", "Low too"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := host.TestSetup(t, host.TestIsolated, tt.opts...)
			c := h.Context(t.Context(), "alice")
			taskSeed(t, c, "None", taskBody("None"), "--column", "up-next")
			taskSeed(t, c, "Low", taskBody("Low"), "--column", "up-next", "--priority", "3")
			taskSeed(t, c, "High", taskBody("High"), "--column", "up-next", "--priority", "1")
			taskSeed(t, c, "Low too", taskBody("Low too"), "--column", "up-next", "--priority", "3")

			resp := run(t, c, "task", nil, "list", "up-next")
			got, ok := data(resp).([]*sdk.Task)
			if !ok {
				t.Fatalf("task list Data = %T, want []*sdk.Task", data(resp))
			}
			if titles := taskTitles(got); !slices.Equal(titles, tt.want) {
				t.Errorf("task list up-next = %v, want %v", titles, tt.want)
			}
			// A negative priority is refused.
			err := runErr(t, c, "task", nil, "set", got[0].Key, "--priority", "-1")
			if !errors.Is(err, sdk.ErrInvalidArg) {
				t.Errorf("task set --priority -1 error = %v, want sdk.ErrInvalidArg", err)
			}
		})
	}
}
