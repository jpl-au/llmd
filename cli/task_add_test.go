package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestTaskAdd covers task creation: the spec sources (stdin, --file,
// --path), every flag, spec path deduplication against documents and
// live tasks, dependency recording, and the error cases.
func TestTaskAdd(t *testing.T) {
	tests := []struct {
		name        string
		noAuthor    bool
		args        []string // after "add"; {file} and {dep} come from setup
		stdin       []byte
		setup       func(t *testing.T, c sdk.Context) taskVars
		wantText    []string
		wantAbsent  []string
		wantErr     error
		wantErrText string
		check       func(t *testing.T, c sdk.Context, tk *sdk.Task, vars taskVars)
	}{
		{
			name:       "title only sits in the backlog without a document",
			args:       []string{"Fix auth"},
			wantText:   []string{"Created task", `"Fix auth" in backlog`},
			wantAbsent: []string{"Spec:"},
			check: func(t *testing.T, c sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Title != "Fix auth" || tk.Status != "backlog" || tk.Path != "tasks/fix-auth" {
					t.Errorf("task = %+v, want Fix auth in backlog at tasks/fix-auth", *tk)
				}
				if ok, _ := c.Documents.Exists(tk.Path); ok {
					t.Errorf("Exists(%s) = true, want no document for a body-less task", tk.Path)
				}
			},
		},
		{
			name:     "stdin body becomes the spec document",
			args:     []string{"Fix auth"},
			stdin:    taskBody("Fix auth"),
			wantText: []string{"Spec: tasks/fix-auth"},
			check: func(t *testing.T, c sdk.Context, tk *sdk.Task, _ taskVars) {
				body, err := c.Documents.Read(tk.Path, sdk.ReadOpts{})
				if err != nil || string(body) != string(taskBody("Fix auth")) {
					t.Errorf("Read(%s) = %q, %v, want the piped body", tk.Path, body, err)
				}
			},
		},
		{
			name:     "multi-word title is joined",
			args:     []string{"Fix", "the", "bug"},
			wantText: []string{`"Fix the bug"`},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Title != "Fix the bug" || tk.Path != "tasks/fix-the-bug" {
					t.Errorf("task = %+v, want title Fix the bug at tasks/fix-the-bug", *tk)
				}
			},
		},
		{
			name:     "--column places the task directly",
			args:     []string{"Fix auth", "--column", "in-progress"},
			stdin:    taskBody("Fix auth"),
			wantText: []string{"in in-progress"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Status != "in-progress" {
					t.Errorf("Status = %q, want in-progress", tk.Status)
				}
			},
		},
		{
			// Spec gating is enforced by move, not add.
			name: "--column without a body is not gated",
			args: []string{"Fix auth", "--column", "review"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Status != "review" {
					t.Errorf("Status = %q, want review", tk.Status)
				}
			},
		},
		{name: "--column unknown", args: []string{"Fix auth", "--column", "nope"}, wantErr: sdk.ErrInvalidArg},
		{
			name: "--priority and --assign",
			args: []string{"Fix auth", "--priority", "3", "--assign", "bob"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Priority != 3 || tk.AssignedTo != "bob" {
					t.Errorf("task = %+v, want priority 3 assigned to bob", *tk)
				}
			},
		},
		{name: "--priority not an integer", args: []string{"Fix auth", "--priority", "high"}, wantErr: sdk.ErrInvalidArg},
		{
			name: "--path uses an existing document",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				writeDoc(t, c, "docs/spec", "# Spec\n\nDetails.")
				return nil
			},
			args:     []string{"Fix auth", "--path", "docs/spec"},
			wantText: []string{"Spec: docs/spec"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Path != "docs/spec" {
					t.Errorf("Path = %q, want docs/spec", tk.Path)
				}
			},
		},
		{name: "--path missing document", args: []string{"Fix auth", "--path", "docs/missing"}, wantErrText: "document not found"},
		{
			name: "--path skips deduplication",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				writeDoc(t, c, "docs/spec", "# Spec\n\nDetails.")
				taskSeed(t, c, "Other", nil, "--path", "docs/spec")
				return nil
			},
			args: []string{"Fix auth", "--path", "docs/spec"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Path != "docs/spec" {
					t.Errorf("Path = %q, want docs/spec shared with the other task", tk.Path)
				}
			},
		},
		{
			name: "--file reads the spec from disk",
			setup: func(t *testing.T, _ sdk.Context) taskVars {
				p := filepath.Join(t.TempDir(), "spec.md")
				if err := os.WriteFile(p, taskBody("Fix auth"), 0o644); err != nil {
					t.Fatal(err)
				}
				return taskVars{"file": p}
			},
			args:     []string{"Fix auth", "--file", "{file}"},
			wantText: []string{"Spec: tasks/fix-auth"},
			check: func(t *testing.T, c sdk.Context, tk *sdk.Task, _ taskVars) {
				body, err := c.Documents.Read(tk.Path, sdk.ReadOpts{})
				if err != nil || string(body) != string(taskBody("Fix auth")) {
					t.Errorf("Read(%s) = %q, %v, want the file contents", tk.Path, body, err)
				}
			},
		},
		{
			name: "--file missing",
			setup: func(t *testing.T, _ sdk.Context) taskVars {
				return taskVars{"file": filepath.Join(t.TempDir(), "missing.md")}
			},
			args:        []string{"Fix auth", "--file", "{file}"},
			wantErrText: "no such file",
		},
		{
			name: "--file and --path are mutually exclusive",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				writeDoc(t, c, "docs/spec", "# Spec\n\nDetails.")
				p := filepath.Join(t.TempDir(), "spec.md")
				if err := os.WriteFile(p, taskBody("Fix auth"), 0o644); err != nil {
					t.Fatal(err)
				}
				return taskVars{"file": p}
			},
			args:        []string{"Fix auth", "--file", "{file}", "--path", "docs/spec"},
			wantErrText: "mutually exclusive",
		},
		{
			name: "path deduplicates against an existing document",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				writeDoc(t, c, "tasks/fix-auth", "# Unrelated\n\nDo not version me.")
				return nil
			},
			args:     []string{"Fix auth"},
			stdin:    taskBody("Fix auth"),
			wantText: []string{"Spec: tasks/fix-auth-2"},
			check: func(t *testing.T, c sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Path != "tasks/fix-auth-2" {
					t.Errorf("Path = %q, want tasks/fix-auth-2", tk.Path)
				}
				body, err := c.Documents.Read("tasks/fix-auth", sdk.ReadOpts{})
				if err != nil || string(body) != "# Unrelated\n\nDo not version me." {
					t.Errorf("Read(tasks/fix-auth) = %q, %v, want the unrelated document untouched", body, err)
				}
			},
		},
		{
			name: "path deduplicates against live tasks",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				taskSeed(t, c, "Fix auth", nil)
				if second := taskSeed(t, c, "Fix auth", nil); second.Path != "tasks/fix-auth-2" {
					t.Fatalf("second task Path = %q, want tasks/fix-auth-2", second.Path)
				}
				return nil
			},
			args: []string{"Fix auth"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Path != "tasks/fix-auth-3" {
					t.Errorf("Path = %q, want tasks/fix-auth-3", tk.Path)
				}
			},
		},
		{
			name: "a deleted task releases its path",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				old := taskSeed(t, c, "Fix auth", nil)
				run(t, c, "task", nil, "rm", old.Key)
				return nil
			},
			args: []string{"Fix auth"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, _ taskVars) {
				if tk.Path != "tasks/fix-auth" {
					t.Errorf("Path = %q, want tasks/fix-auth reused after rm", tk.Path)
				}
			},
		},
		{
			name: "--depends-on records the dependency",
			setup: func(t *testing.T, c sdk.Context) taskVars {
				return taskVars{"dep": taskSeed(t, c, "Base", nil).Key}
			},
			args: []string{"Fix auth", "--depends-on", "{dep}"},
			check: func(t *testing.T, _ sdk.Context, tk *sdk.Task, vars taskVars) {
				if tk.DependsOn != vars["dep"] {
					t.Errorf("DependsOn = %q, want %q", tk.DependsOn, vars["dep"])
				}
			},
		},
		{name: "--depends-on missing task", args: []string{"Fix auth", "--depends-on", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no title", args: nil, wantErr: sdk.ErrMissingArg},
		{name: "empty title", args: []string{""}, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", args: []string{"Fix auth", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "no author", noAuthor: true, args: []string{"Fix auth"}, wantErrText: "author required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			author := "alice"
			if tt.noAuthor {
				author = ""
			}
			c := newCLI(t, author)
			var vars taskVars
			if tt.setup != nil {
				vars = tt.setup(t, c)
			}
			args := append([]string{"add"}, taskExpand(tt.args, vars)...)

			if tt.wantErr != nil || tt.wantErrText != "" {
				taskWantErr(t, "task", args, runErr(t, c, "task", tt.stdin, args...), tt.wantErr, tt.wantErrText)
				return
			}
			resp := run(t, c, "task", tt.stdin, args...)
			tk, ok := data(resp).(*sdk.Task)
			if !ok || tk == nil {
				t.Fatalf("task %v Data = %T, want *sdk.Task", args, data(resp))
			}
			taskWantText(t, "task", args, text(resp), append([]string{"Created task " + tk.Key}, tt.wantText...), tt.wantAbsent)
			if tk.Author != "alice" {
				t.Errorf("Author = %q, want alice", tk.Author)
			}
			if stored := taskRead(t, c, tk.Key); stored.Path != tk.Path || stored.Status != tk.Status {
				t.Errorf("stored task = %+v, want %+v", *stored, *tk)
			}
			if tt.check != nil {
				tt.check(t, c, tk, vars)
			}
		})
	}
}
