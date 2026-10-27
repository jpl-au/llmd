package cli

import (
	"maps"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// TestTaskShow covers the markdown detail view: the metadata table,
// the optional rows (assignee, branch, dependency, flags), the spec
// body, and the error cases. Each case seeds "Alpha" and may mutate
// it before showing.
func TestTaskShow(t *testing.T) {
	tests := []struct {
		name       string
		noAuthor   bool
		noBody     bool // seed Alpha without a spec document
		setup      func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars
		args       []string // defaults to {"show", "{key}"}
		wantText   []string
		wantAbsent []string
		wantErr    error
	}{
		{
			name:       "metadata table and spec body",
			wantText:   []string{"# Alpha", "| ID | {key} |", "| Status | backlog |", "| Priority | 0 |", "| Spec | tasks/alpha |", "---", "Alpha spec body."},
			wantAbsent: []string{"Assigned To", "Branch", "Depends On", "Flags"},
		},
		{
			name:       "no document shows metadata only",
			noBody:     true,
			wantText:   []string{"# Alpha", "| Spec | tasks/alpha |"},
			wantAbsent: []string{"spec body"},
		},
		{
			name: "assignee and priority",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--assign", "bob", "--priority", "2")
				return nil
			},
			wantText: []string{"| Assigned To | bob |", "| Priority | 2 |"},
		},
		{
			name: "flags",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--flag", "blocked")
				return nil
			},
			wantText: []string{"| Flags | blocked |"},
		},
		{
			name: "branch",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "set", tk.Key, "--branch", "feature-x")
				return nil
			},
			wantText: []string{"| Branch | feature-x"},
		},
		{
			name: "dependency",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				dep := taskSeed(t, c, "Base", nil)
				run(t, c, "task", nil, "set", tk.Key, "--depends-on", dep.Key)
				return taskVars{"dep": dep.Key}
			},
			wantText: []string{"| Depends On | {dep} |"},
		},
		{
			name: "moved task reports its column",
			setup: func(t *testing.T, c sdk.Context, tk *sdk.Task) taskVars {
				run(t, c, "task", nil, "move", tk.Key, "in-progress")
				return nil
			},
			wantText: []string{"| Status | in-progress |"},
		},
		{name: "reads without author", noAuthor: true, wantText: []string{"# Alpha"}},
		{name: "missing task", args: []string{"show", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", args: []string{"show"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := host.TestSetup(t, host.TestIsolated)
			c := h.Context(t.Context(), "alice")
			body := taskBody("Alpha")
			if tt.noBody {
				body = nil
			}
			seed := taskSeed(t, c, "Alpha", body)
			vars := taskVars{"key": seed.Key}
			if tt.setup != nil {
				maps.Copy(vars, tt.setup(t, c, seed))
			}
			if tt.noAuthor {
				c = h.Context(t.Context(), "")
			}
			args := []string{"show", "{key}"}
			if tt.args != nil {
				args = tt.args
			}
			args = taskExpand(args, vars)

			if tt.wantErr != nil {
				taskWantErr(t, "task", args, runErr(t, c, "task", nil, args...), tt.wantErr, "")
				return
			}
			resp := run(t, c, "task", nil, args...)
			if _, ok := resp.(sdk.Markdown); !ok {
				t.Errorf("task %v response type = %T, want sdk.Markdown", args, resp)
			}
			taskWantText(t, "task", args, text(resp), taskExpand(tt.wantText, vars), tt.wantAbsent)
			tk, ok := data(resp).(*sdk.Task)
			if !ok || tk == nil || tk.Key != seed.Key {
				t.Errorf("task %v Data = %+v, want the task %s", args, data(resp), seed.Key)
			}
		})
	}
}
