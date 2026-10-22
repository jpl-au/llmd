package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// taskDefaultColumns is the board a fresh store starts with.
var taskDefaultColumns = []string{"backlog", "up-next", "in-progress", "review", "approval", "done", "blocked"}

// taskBoardColumns returns the board columns as stored.
func taskBoardColumns(t *testing.T, c sdk.Context) []string {
	t.Helper()
	cols, err := c.Tasks.Columns()
	if err != nil {
		t.Fatalf("Tasks.Columns: %v", err)
	}
	return cols
}

// TestTaskColumnList covers the column listing and its aliases.
func TestTaskColumnList(t *testing.T) {
	tests := []struct {
		name     string
		noAuthor bool
		args     []string
	}{
		{name: "column list", args: []string{"column", "list"}},
		{name: "column ls", args: []string{"column", "ls"}},
		{name: "bare column", args: []string{"column"}},
		{name: "reads without author", noAuthor: true, args: []string{"column", "list"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			if tt.noAuthor {
				c.Author = ""
			}
			resp := run(t, c, "task", nil, tt.args...)
			if got, want := text(resp), strings.Join(taskDefaultColumns, "\n"); got != want {
				t.Errorf("task %v = %q, want %q", tt.args, got, want)
			}
			if got, ok := data(resp).([]string); !ok || !slices.Equal(got, taskDefaultColumns) {
				t.Errorf("task %v Data = %v, want %v", tt.args, data(resp), taskDefaultColumns)
			}
		})
	}
}

// TestTaskColumnAdd covers appending and --after placement, plus the
// duplicate, unknown anchor and argument errors. Column mutations
// sync .llmd/rules relative to the cwd, so the store is the cwd.
func TestTaskColumnAdd(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		args        []string
		wantCols    []string
		wantErr     error
		wantErrText string
	}{
		{
			name:     "appends by default",
			args:     []string{"column", "add", "testing"},
			wantCols: append(slices.Clone(taskDefaultColumns), "testing"),
		},
		{
			name:     "--after inserts in place",
			args:     []string{"column", "add", "testing", "--after", "in-progress"},
			wantCols: []string{"backlog", "up-next", "in-progress", "testing", "review", "approval", "done", "blocked"},
		},
		{
			name:     "--after the last column",
			args:     []string{"column", "add", "testing", "--after", "blocked"},
			wantCols: append(slices.Clone(taskDefaultColumns), "testing"),
		},
		{name: "duplicate", args: []string{"column", "add", "review"}, wantErr: sdk.ErrExists},
		{name: "--after unknown column", args: []string{"column", "add", "testing", "--after", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no name", args: []string{"column", "add"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", args: []string{"column", "add", "testing", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "no author", noAuthor: true, args: []string{"column", "add", "testing"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := taskWorkdirCLI(t, "alice")
			if tt.noAuthor {
				c.Author = ""
			}
			if taskExpectErr(t, c, "task", tt.args, tt.wantErr, tt.wantErrText) {
				if got := taskBoardColumns(t, c); !slices.Equal(got, taskDefaultColumns) {
					t.Errorf("Columns after failed add = %v, want defaults", got)
				}
				return
			}
			resp := run(t, c, "task", nil, tt.args...)
			taskWantText(t, "task", tt.args, text(resp), []string{"Added column testing"}, nil)
			if got := taskBoardColumns(t, c); !slices.Equal(got, tt.wantCols) {
				t.Errorf("Columns = %v, want %v", got, tt.wantCols)
			}
			// The new column is usable straight away.
			if tk := taskSeed(t, c, "Alpha", nil, "--column", "testing"); tk.Status != "testing" {
				t.Errorf("task added to the new column has Status %q", tk.Status)
			}
		})
	}
}

// TestTaskColumnRm covers removing an empty column, the refusal to
// remove one that still holds tasks, and the argument errors.
func TestTaskColumnRm(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		occupied    bool // seed a task in "approval" first
		args        []string
		wantCols    []string
		wantErr     error
		wantErrText string
	}{
		{
			name:     "empty column",
			args:     []string{"column", "rm", "approval"},
			wantCols: []string{"backlog", "up-next", "in-progress", "review", "done", "blocked"},
		},
		{name: "column with tasks", occupied: true, args: []string{"column", "rm", "approval"}, wantErr: sdk.ErrInvalidArg},
		{name: "column with tasks names the count", occupied: true, args: []string{"column", "rm", "approval"}, wantErrText: "(1 tasks)"},
		{name: "unknown column", args: []string{"column", "rm", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no name", args: []string{"column", "rm"}, wantErr: sdk.ErrMissingArg},
		{name: "no author", noAuthor: true, args: []string{"column", "rm", "approval"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := taskWorkdirCLI(t, "alice")
			if tt.occupied {
				taskSeed(t, c, "Alpha", nil, "--column", "approval")
			}
			if tt.noAuthor {
				c.Author = ""
			}
			if taskExpectErr(t, c, "task", tt.args, tt.wantErr, tt.wantErrText) {
				if got := taskBoardColumns(t, c); !slices.Equal(got, taskDefaultColumns) {
					t.Errorf("Columns after failed rm = %v, want defaults", got)
				}
				return
			}
			resp := run(t, c, "task", nil, tt.args...)
			taskWantText(t, "task", tt.args, text(resp), []string{"Removed column approval"}, nil)
			if got := taskBoardColumns(t, c); !slices.Equal(got, tt.wantCols) {
				t.Errorf("Columns = %v, want %v", got, tt.wantCols)
			}
		})
	}
}

// TestTaskColumnMv covers reordering with --after, the move alias,
// the required-flag and unknown-column errors.
func TestTaskColumnMv(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		args        []string
		wantText    string
		wantCols    []string
		wantErr     error
		wantErrText string
	}{
		{
			name:     "moves after the anchor",
			args:     []string{"column", "mv", "approval", "--after", "backlog"},
			wantText: "Moved column approval after backlog",
			wantCols: []string{"backlog", "approval", "up-next", "in-progress", "review", "done", "blocked"},
		},
		{
			name:     "move alias",
			args:     []string{"column", "move", "backlog", "--after", "blocked"},
			wantText: "Moved column backlog after blocked",
			wantCols: []string{"up-next", "in-progress", "review", "approval", "done", "blocked", "backlog"},
		},
		{
			name:     "--after=value form",
			args:     []string{"column", "mv", "done", "--after=backlog"},
			wantText: "Moved column done after backlog",
			wantCols: []string{"backlog", "done", "up-next", "in-progress", "review", "approval", "blocked"},
		},
		{name: "--after is required", args: []string{"column", "mv", "approval"}, wantErrText: "--after is required"},
		{name: "unknown column", args: []string{"column", "mv", "nope", "--after", "backlog"}, wantErr: sdk.ErrNotFound},
		{name: "unknown anchor", args: []string{"column", "mv", "approval", "--after", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no name", args: []string{"column", "mv", "--after", "backlog"}, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", args: []string{"column", "mv", "approval", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "no author", noAuthor: true, args: []string{"column", "mv", "approval", "--after", "backlog"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := taskWorkdirCLI(t, "alice")
			if tt.noAuthor {
				c.Author = ""
			}
			if taskExpectErr(t, c, "task", tt.args, tt.wantErr, tt.wantErrText) {
				if got := taskBoardColumns(t, c); !slices.Equal(got, taskDefaultColumns) {
					t.Errorf("Columns after failed mv = %v, want defaults", got)
				}
				return
			}
			resp := run(t, c, "task", nil, tt.args...)
			if got := text(resp); got != tt.wantText {
				t.Errorf("task %v = %q, want %q", tt.args, got, tt.wantText)
			}
			if got := taskBoardColumns(t, c); !slices.Equal(got, tt.wantCols) {
				t.Errorf("Columns = %v, want %v", got, tt.wantCols)
			}
		})
	}
}
