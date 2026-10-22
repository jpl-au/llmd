package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestTaskMove covers column changes and spec gating: a task leaves
// the backlog only when its document has content beyond the title
// heading, while moves within the backlog or from any other column
// are not gated.
func TestTaskMove(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		seedBody    []byte // nil seeds Alpha without a document
		seedColumn  string // "" is backlog
		args        []string
		wantStatus  string
		wantErr     error
		wantErrText []string
		skip        string
	}{
		{
			name:       "a spec lets a task leave the backlog",
			seedBody:   taskBody("Alpha"),
			args:       []string{"move", "{key}", "in-progress"},
			wantStatus: "in-progress",
		},
		{
			name:        "title-only document is rejected with guidance",
			seedBody:    []byte("# Alpha"),
			args:        []string{"move", "{key}", "in-progress"},
			wantErrText: []string{"spec required", "llmd write tasks/alpha", "llmd task link {key}"},
		},
		{
			name:        "no document is rejected",
			args:        []string{"move", "{key}", "in-progress"},
			wantErrText: []string{"spec required"},
		},
		{
			name:    "spec gating keeps the ErrNoSpec sentinel",
			args:    []string{"move", "{key}", "in-progress"},
			wantErr: sdk.ErrNoSpec,
		},
		{
			name:       "backlog to backlog needs no spec",
			args:       []string{"move", "{key}", "backlog"},
			wantStatus: "backlog",
		},
		{
			name:       "gating only applies when leaving the backlog",
			seedColumn: "in-progress",
			args:       []string{"move", "{key}", "review"},
			wantStatus: "review",
		},
		{
			name:       "done is reachable",
			seedBody:   taskBody("Alpha"),
			args:       []string{"move", "{key}", "done"},
			wantStatus: "done",
		},
		{name: "unknown column", seedBody: taskBody("Alpha"), args: []string{"move", "{key}", "nope"}, wantErr: sdk.ErrInvalidArg},
		{name: "missing task", args: []string{"move", "nope", "done"}, wantErr: sdk.ErrNotFound},
		{name: "missing column", args: []string{"move", "{key}"}, wantErr: sdk.ErrMissingArg},
		{name: "no args", args: []string{"move"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, seedBody: taskBody("Alpha"), args: []string{"move", "{key}", "in-progress"}, wantErrText: []string{"author required"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			var addArgs []string
			if tt.seedColumn != "" {
				addArgs = []string{"--column", tt.seedColumn}
			}
			seed := taskSeed(t, c, "Alpha", tt.seedBody, addArgs...)
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, taskVars{"key": seed.Key})

			if tt.wantErr != nil || len(tt.wantErrText) > 0 {
				err := runErr(t, c, "task", nil, args...)
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("task %v error = %v, want %v", args, err, tt.wantErr)
				}
				for _, want := range taskExpand(tt.wantErrText, taskVars{"key": seed.Key}) {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("task %v error = %v, want to contain %q", args, err, want)
					}
				}
				if got := taskRead(t, c, seed.Key).Status; got != seed.Status {
					t.Errorf("Status after failed move = %q, want %q unchanged", got, seed.Status)
				}
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("task %v response type = %T, want sdk.Text", args, resp)
			}
			taskWantText(t, "task", args, text(resp), []string{"Moved task " + seed.Key + " to " + tt.wantStatus}, nil)
			if got := taskRead(t, c, seed.Key).Status; got != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

// TestTaskSet covers every metadata flag, clearing fields with an
// empty value, repositioning within a column, dependency validation
// (missing target, self and two-task cycles) and the error cases.
func TestTaskSet(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		setup       func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars
		args        []string // {key} is Alpha; others come from setup
		wantErr     error
		wantErrText string
		check       func(t *testing.T, c sdk.Context, tk *sdk.Task, vars taskVars)
	}{
		{
			name: "--title",
			args: []string{"set", "{key}", "--title", "Renamed"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Title != "Renamed" {
					t.Errorf("Title = %q, want Renamed", tk.Title)
				}
			},
		},
		{
			name: "--priority",
			args: []string{"set", "{key}", "--priority", "5"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Priority != 5 {
					t.Errorf("Priority = %d, want 5", tk.Priority)
				}
			},
		},
		{
			name: "--assign",
			args: []string{"set", "{key}", "--assign", "bob"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.AssignedTo != "bob" {
					t.Errorf("AssignedTo = %q, want bob", tk.AssignedTo)
				}
			},
		},
		{
			name: "--assign empty unassigns",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--assign", "bob")
				return nil
			},
			args: []string{"set", "{key}", "--assign", ""},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.AssignedTo != "" {
					t.Errorf("AssignedTo = %q, want empty", tk.AssignedTo)
				}
			},
		},
		{
			name: "--position reorders within the column",
			setup: func(t *testing.T, c sdk.Context, _ *sdk.Task) taskVars {
				return taskVars{"beta": taskSeed(t, c, "Beta", nil).Key}
			},
			args: []string{"set", "{beta}", "--position", "0"},
			check: func(t *testing.T, c sdk.Context, tk *sdk.Task, vars taskVars) {
				beta := taskRead(t, c, vars["beta"])
				if beta.Position != 0 || tk.Position != 1 {
					t.Errorf("positions beta=%d alpha=%d, want 0 and 1", beta.Position, tk.Position)
				}
			},
		},
		{
			name: "--flag",
			args: []string{"set", "{key}", "--flag", "blocked"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Flags != "blocked" {
					t.Errorf("Flags = %q, want blocked", tk.Flags)
				}
			},
		},
		{
			name: "--flag accumulates",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--flag", "blocked")
				return nil
			},
			args: []string{"set", "{key}", "--flag", "hold"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if !strings.Contains(tk.Flags, "blocked") || !strings.Contains(tk.Flags, "hold") {
					t.Errorf("Flags = %q, want both blocked and hold", tk.Flags)
				}
			},
		},
		{
			name: "--unflag",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--flag", "blocked")
				return nil
			},
			args: []string{"set", "{key}", "--unflag", "blocked"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Flags != "" {
					t.Errorf("Flags = %q, want empty", tk.Flags)
				}
			},
		},
		{
			name: "--unflag of an absent flag is a no-op",
			args: []string{"set", "{key}", "--unflag", "hold"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Flags != "" {
					t.Errorf("Flags = %q, want empty", tk.Flags)
				}
			},
		},
		{
			name: "--branch",
			args: []string{"set", "{key}", "--branch", "feature-x"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Branch != "feature-x" {
					t.Errorf("Branch = %q, want feature-x", tk.Branch)
				}
			},
		},
		{
			name: "--branch empty clears it",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--branch", "feature-x")
				return nil
			},
			args: []string{"set", "{key}", "--branch", ""},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Branch != "" {
					t.Errorf("Branch = %q, want empty", tk.Branch)
				}
			},
		},
		{
			name: "--depends-on",
			setup: func(t *testing.T, c sdk.Context, _ *sdk.Task) taskVars {
				return taskVars{"dep": taskSeed(t, c, "Base", nil).Key}
			},
			args: []string{"set", "{key}", "--depends-on", "{dep}"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, vars taskVars) {
				if tk.DependsOn != vars["dep"] {
					t.Errorf("DependsOn = %q, want %q", tk.DependsOn, vars["dep"])
				}
			},
		},
		{
			name: "--depends-on empty clears it",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				dep := taskSeed(t, c, "Base", nil)
				run(t, c, "task", nil, "set", tk.Key, "--depends-on", dep.Key)
				return nil
			},
			args: []string{"set", "{key}", "--depends-on", ""},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.DependsOn != "" {
					t.Errorf("DependsOn = %q, want empty", tk.DependsOn)
				}
			},
		},
		{name: "--depends-on self", args: []string{"set", "{key}", "--depends-on", "{key}"}, wantErr: sdk.ErrCycle},
		{
			name: "--depends-on two-task cycle",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				dep := taskSeed(t, c, "Base", nil)
				run(t, c, "task", nil, "set", tk.Key, "--depends-on", dep.Key)
				return taskVars{"dep": dep.Key}
			},
			args:    []string{"set", "{dep}", "--depends-on", "{key}"},
			wantErr: sdk.ErrCycle,
		},
		{name: "--depends-on missing task", args: []string{"set", "{key}", "--depends-on", "nope"}, wantErr: sdk.ErrNotFound},
		{
			name: "several fields at once",
			args: []string{"set", "{key}", "--title", "Renamed", "--priority", "3", "--assign", "bob", "--flag", "hold"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Title != "Renamed" || tk.Priority != 3 || tk.AssignedTo != "bob" || tk.Flags != "hold" {
					t.Errorf("task = %+v, want Renamed/3/bob/hold", *tk)
				}
			},
		},
		{
			name: "no flags is a no-op",
			args: []string{"set", "{key}"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Title != "Alpha" || tk.Priority != 0 || tk.Flags != "" {
					t.Errorf("task = %+v, want unchanged", *tk)
				}
			},
		},
		{name: "--priority not an integer", args: []string{"set", "{key}", "--priority", "high"}, wantErr: sdk.ErrInvalidArg},
		{name: "--position not an integer", args: []string{"set", "{key}", "--position", "top"}, wantErr: sdk.ErrInvalidArg},
		{name: "unknown flag", args: []string{"set", "{key}", "--bogus", "x"}, wantErr: sdk.ErrInvalidArg},
		{name: "missing task", args: []string{"set", "nope", "--priority", "1"}, wantErr: sdk.ErrNotFound},
		{name: "no id", args: []string{"set"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, args: []string{"set", "{key}", "--priority", "1"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			seed := taskSeed(t, c, "Alpha", nil)
			vars := taskVars{"key": seed.Key}
			if tt.setup != nil {
				for k, v := range tt.setup(t, c, seed) {
					vars[k] = v
				}
			}
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, vars)

			if taskExpectErr(t, c, "task", args, tt.wantErr, tt.wantErrText) {
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("task %v response type = %T, want sdk.Text", args, resp)
			}
			taskWantText(t, "task", args, text(resp), []string{"Updated task " + args[1]}, nil)
			if tt.check != nil {
				tt.check(t, c, taskRead(t, c, seed.Key), vars)
			}
		})
	}
}

// TestTaskRm covers soft deletion: the confirmation names the task,
// reminds the user when the spec document survives, and the task is
// gone from reads afterwards.
func TestTaskRm(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		seedBody    []byte
		args        []string
		wantText    []string
		wantAbsent  []string
		wantErr     error
		wantErrText string
	}{
		{
			name:     "with a document reminds that it survives",
			seedBody: taskBody("Alpha"),
			args:     []string{"rm", "{key}"},
			wantText: []string{`Removed task {key} "Alpha"`, "Note: the document at tasks/alpha still exists", "llmd rm tasks/alpha"},
		},
		{
			name:       "without a document",
			args:       []string{"rm", "{key}"},
			wantText:   []string{`Removed task {key} "Alpha"`},
			wantAbsent: []string{"Note:"},
		},
		{name: "missing task", args: []string{"rm", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", args: []string{"rm"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, args: []string{"rm", "{key}"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			seed := taskSeed(t, c, "Alpha", tt.seedBody)
			if tt.noAuthor {
				c.Author = ""
			}
			vars := taskVars{"key": seed.Key}
			args := taskExpand(tt.args, vars)

			if taskExpectErr(t, c, "task", args, tt.wantErr, tt.wantErrText) {
				taskRead(t, c, seed.Key) // still live
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("task %v response type = %T, want sdk.Text", args, resp)
			}
			taskWantText(t, "task", args, text(resp), taskExpand(tt.wantText, vars), tt.wantAbsent)

			if _, err := c.Tasks.Read(seed.Key); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("Read after rm error = %v, want %v", err, sdk.ErrNotFound)
			}
			if err := runErr(t, c, "task", nil, "rm", seed.Key); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("second rm error = %v, want %v", err, sdk.ErrNotFound)
			}
			if tt.seedBody != nil {
				if ok, err := c.Documents.Exists(seed.Path); !ok || err != nil {
					t.Errorf("Exists(%s) = %v, %v, want the document untouched", seed.Path, ok, err)
				}
			}
		})
	}
}

// TestTaskRestore covers undeleting: the task returns to the column it
// was in, and unknown keys are ErrNotFound.
func TestTaskRestore(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		live        bool // do not delete the seeded task first
		seedColumn  string
		args        []string
		wantText    []string
		wantErr     error
		wantErrText string
	}{
		{name: "restores to the original column", seedColumn: "in-progress", args: []string{"restore", "{key}"}, wantText: []string{`Restored task {key} "Alpha"`}},
		{name: "live task is not found", live: true, args: []string{"restore", "{key}"}, wantErr: sdk.ErrNotFound},
		{name: "restores a backlog task", args: []string{"restore", "{key}"}, wantText: []string{`Restored task {key} "Alpha"`}},
		{name: "missing task", args: []string{"restore", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", args: []string{"restore"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, args: []string{"restore", "{key}"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			var addArgs []string
			if tt.seedColumn != "" {
				addArgs = []string{"--column", tt.seedColumn}
			}
			seed := taskSeed(t, c, "Alpha", taskBody("Alpha"), addArgs...)
			if !tt.live {
				run(t, c, "task", nil, "rm", seed.Key)
			}
			if tt.noAuthor {
				c.Author = ""
			}
			vars := taskVars{"key": seed.Key}
			args := taskExpand(tt.args, vars)

			if taskExpectErr(t, c, "task", args, tt.wantErr, tt.wantErrText) {
				_, err := c.Tasks.Read(seed.Key)
				if tt.live && err != nil {
					t.Errorf("Read after rejected restore of a live task error = %v, want the task unchanged", err)
				}
				if !tt.live && !errors.Is(err, sdk.ErrNotFound) {
					t.Errorf("Read after failed restore error = %v, want still deleted", err)
				}
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("task %v response type = %T, want sdk.Text", args, resp)
			}
			taskWantText(t, "task", args, text(resp), taskExpand(tt.wantText, vars), nil)
			if got := taskRead(t, c, seed.Key); got.Status != seed.Status || got.Title != "Alpha" {
				t.Errorf("restored task = %+v, want status %q", *got, seed.Status)
			}
		})
	}
}
