package host

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/sdk"
)

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

// gitHost creates a store and a git repository with one commit on
// main in the same directory, with the working directory set to it.
func gitHost(t *testing.T) (*Host, sdk.Context) {
	t.Helper()
	return gitHostOn(t, "main")
}

// gitHostOn is gitHost with the initial branch named.
func gitHostOn(t *testing.T, branch string) (*Host, sdk.Context) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	h := TestSetup(t, TestWorkdir)
	dir := h.Dir()
	runGit(t, dir, "init", "-b", branch)
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")
	return h, h.Context(t.Context(), "alice")
}

// commitFile writes a file and commits it in dir.
func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-m", "add "+name)
}

// TestTasksStartBranch covers branch naming, the column, the rejection
// of a task that already has a branch, and running outside a git
// repository.
func TestTasksStartBranch(t *testing.T) {
	tests := []struct {
		name       string
		git        bool
		title      string
		body       []byte
		taskOpts   sdk.TaskAddOpts
		setup      func(t *testing.T, dir string)
		opts       sdk.StartBranchOpts
		wantBranch string
		wantStatus string
		wantErr    error
		wantAny    bool
	}{
		{name: "branch from the title", git: true, title: "Fix auth tokens!", body: spec, opts: sdk.StartBranchOpts{}, wantBranch: "task/fix-auth-tokens", wantStatus: "in-progress"},
		{name: "custom name and column", git: true, title: "Fix auth", body: spec, opts: sdk.StartBranchOpts{Name: "feature/auth", Column: "review"}, wantBranch: "feature/auth", wantStatus: "review"},
		{name: "task already has a branch", git: true, title: "Fix auth", body: spec, taskOpts: sdk.TaskAddOpts{Branch: "existing"}, wantErr: sdk.ErrInvalidArg},
		{
			name: "branch already exists in git", git: true, title: "Fix auth", body: spec,
			setup:   func(t *testing.T, dir string) { runGit(t, dir, "branch", "task/fix-auth") },
			wantAny: true,
		},
		{name: "task without a spec cannot start", git: true, title: "Fix auth", wantErr: sdk.ErrNoSpec},
		{name: "no git repository", git: false, title: "Fix auth", body: spec, wantAny: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *Host
			var c sdk.Context
			if tt.git {
				h, c = gitHost(t)
			} else {
				h = TestSetup(t, TestWorkdir)
				c = h.Context(t.Context(), "alice")
			}
			if tt.setup != nil {
				tt.setup(t, h.Dir())
			}
			opts := tt.taskOpts
			opts.Author = "alice"
			task := addTask(t, c, tt.title, tt.body, opts)

			got, err := c.Tasks.StartBranch(task.Key, "alice", tt.opts)
			switch {
			case tt.wantAny:
				if err == nil {
					t.Fatalf("StartBranch(%+v) error = nil, want an error", tt.opts)
				}
				return
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("StartBranch(%+v) error = %v, want %v", tt.opts, err, tt.wantErr)
				}
				stored := readTask(t, c, task.Key)
				if stored.Status != "backlog" || stored.Branch != task.Branch {
					t.Errorf("task after failed StartBranch = status %q branch %q, want backlog and branch %q unchanged", stored.Status, stored.Branch, task.Branch)
				}
				if tt.git {
					if current := runGit(t, h.Dir(), "branch", "--show-current"); current != "main" {
						t.Errorf("git branch after failed StartBranch = %q, want main", current)
					}
				}
				return
			case err != nil:
				t.Fatalf("StartBranch(%+v) error = %v", tt.opts, err)
			}
			if got.Branch != tt.wantBranch || got.Status != tt.wantStatus {
				t.Errorf("StartBranch(%+v) = branch %q status %q, want %q %q", tt.opts, got.Branch, got.Status, tt.wantBranch, tt.wantStatus)
			}
			if current := runGit(t, h.Dir(), "branch", "--show-current"); current != tt.wantBranch {
				t.Errorf("git branch after StartBranch = %q, want %q", current, tt.wantBranch)
			}
			if stored := readTask(t, c, task.Key); stored.Branch != tt.wantBranch {
				t.Errorf("Read().Branch = %q, want %q", stored.Branch, tt.wantBranch)
			}
		})
	}
}

// TestTasksFinishGit covers the git summary on finish: changed files
// and commits on the task's branch against the detected or given
// base, zero counts when the branch is unknown, when there is no
// branch, and when there is no git, and the worktree cleanup attempt
// for a task with an agent run.
func TestTasksFinishGit(t *testing.T) {
	tests := []struct {
		name    string
		git     bool
		initial string                                                 // initial git branch; main when empty
		branch  string                                                 // recorded on the task
		setup   func(t *testing.T, h *Host, c sdk.Context, key string) // after the task exists
		opts    sdk.FinishOpts
		wantRes sdk.FinishResult
	}{
		{
			name:    "no default branch detected",
			git:     true,
			initial: "trunk",
			branch:  "feature/work",
			setup: func(t *testing.T, h *Host, _ sdk.Context, _ string) {
				dir := h.Dir()
				runGit(t, dir, "checkout", "-b", "feature/work")
				commitFile(t, dir, "a.txt", "a")
			},
			wantRes: sdk.FinishResult{},
		},
		{
			name:   "summary against the default branch",
			git:    true,
			branch: "feature/work",
			setup: func(t *testing.T, h *Host, _ sdk.Context, _ string) {
				dir := h.Dir()
				runGit(t, dir, "checkout", "-b", "feature/work")
				commitFile(t, dir, "a.txt", "a")
				commitFile(t, dir, "b.txt", "b")
			},
			wantRes: sdk.FinishResult{FilesChanged: 2, Commits: 2},
		},
		{
			name:   "explicit base",
			git:    true,
			branch: "feature/work",
			setup: func(t *testing.T, h *Host, _ sdk.Context, _ string) {
				dir := h.Dir()
				runGit(t, dir, "checkout", "-b", "release")
				commitFile(t, dir, "r.txt", "r")
				runGit(t, dir, "checkout", "-b", "feature/work")
				commitFile(t, dir, "a.txt", "a")
			},
			opts:    sdk.FinishOpts{Base: "release"},
			wantRes: sdk.FinishResult{FilesChanged: 1, Commits: 1},
		},
		{
			name:    "unknown branch",
			git:     true,
			branch:  "feature/nowhere",
			wantRes: sdk.FinishResult{},
		},
		{
			name:    "no branch recorded",
			git:     true,
			wantRes: sdk.FinishResult{},
		},
		{
			name:    "no git repository",
			git:     false,
			branch:  "feature/work",
			wantRes: sdk.FinishResult{},
		},
		{
			name:   "agent worktree cleanup is attempted",
			git:    true,
			branch: "feature/work",
			setup: func(t *testing.T, h *Host, _ sdk.Context, key string) {
				dir := h.Dir()
				runGit(t, dir, "checkout", "-b", "feature/work")
				commitFile(t, dir, "a.txt", "a")
				seedRun(t, h, agents.RecordOpts{TaskKey: key, Agent: "claude-code", Worktree: filepath.Join(dir, "not-a-worktree")})
			},
			wantRes: sdk.FinishResult{FilesChanged: 1, Commits: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *Host
			var c sdk.Context
			switch {
			case tt.git && tt.initial != "":
				h, c = gitHostOn(t, tt.initial)
			case tt.git:
				h, c = gitHost(t)
			default:
				h = TestSetup(t, TestWorkdir)
				c = h.Context(t.Context(), "alice")
			}
			task := addTask(t, c, "Finish me", spec, sdk.TaskAddOpts{Author: "alice", Branch: tt.branch})
			moveTask(t, c, task.Key, "in-progress")
			if tt.setup != nil {
				tt.setup(t, h, c, task.Key)
			}

			got, err := c.Tasks.Finish(task.Key, "alice", tt.opts)
			if err != nil {
				t.Fatalf("Finish(%+v) error = %v", tt.opts, err)
			}
			if got.Task.Status != "done" || got.Task.Key != task.Key {
				t.Errorf("Finish(%+v).Task = %+v, want %s in done", tt.opts, got.Task, task.Key)
			}
			if got.FilesChanged != tt.wantRes.FilesChanged || got.Commits != tt.wantRes.Commits {
				t.Errorf("Finish(%+v) = files %d commits %d, want %d %d", tt.opts, got.FilesChanged, got.Commits, tt.wantRes.FilesChanged, tt.wantRes.Commits)
			}
		})
	}
}

// TestTasksLink covers linking a task's spec to a document and the
// errors for a missing target, a self link and a duplicate.
func TestTasksLink(t *testing.T) {
	tests := []struct {
		name    string
		target  func(task *sdk.Task) string
		setup   func(t *testing.T, c sdk.Context, task *sdk.Task)
		wantErr error
	}{
		{
			name:   "links to a document",
			target: func(*sdk.Task) string { return "docs/ref" },
			setup:  func(t *testing.T, c sdk.Context, _ *sdk.Task) { writeDoc(t, c, "docs/ref", "ref") },
		},
		{
			name:    "missing target",
			target:  func(*sdk.Task) string { return "docs/missing" },
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "self link",
			target:  func(task *sdk.Task) string { return task.Path },
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:   "duplicate",
			target: func(*sdk.Task) string { return "docs/ref" },
			setup: func(t *testing.T, c sdk.Context, task *sdk.Task) {
				writeDoc(t, c, "docs/ref", "ref")
				if err := c.Tasks.Link(task.Key, "docs/ref", "alice"); err != nil {
					t.Fatalf("Link: %v", err)
				}
			},
			wantErr: sdk.ErrExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Linked", spec, sdk.TaskAddOpts{Author: "alice"})
			if tt.setup != nil {
				tt.setup(t, c, task)
			}
			target := tt.target(task)

			err := c.Tasks.Link(task.Key, target, "alice")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Link(%q, %q) error = %v, want %v", task.Key, target, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			links, err := c.Links.List(task.Path, "out")
			if err != nil {
				t.Fatalf("Links.List(%q) error = %v", task.Path, err)
			}
			if len(links) != 1 || links[0].From != task.Path || links[0].To != target || links[0].Label != "" {
				t.Errorf("Links.List(%q) = %+v, want one unlabelled link to %s", task.Path, links, target)
			}
		})
	}
}

// TestTasksLinks covers listing a task's links in each direction.
func TestTasksLinks(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want []sdk.Link // From/To filled with {spec} for the task's path
	}{
		{"outgoing", "out", []sdk.Link{{From: "{spec}", To: "docs/out", Label: "see"}}},
		{"incoming", "in", []sdk.Link{{From: "docs/in", To: "{spec}", Label: "cites"}}},
		{"both", "both", []sdk.Link{{From: "docs/in", To: "{spec}", Label: "cites"}, {From: "{spec}", To: "docs/out", Label: "see"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			task := addTask(t, c, "Linked", spec, sdk.TaskAddOpts{Author: "alice"})
			writeDoc(t, c, "docs/out", "out")
			writeDoc(t, c, "docs/in", "in")
			if err := c.Links.Add(task.Path, "docs/out", "see", "alice"); err != nil {
				t.Fatalf("Links.Add(out): %v", err)
			}
			if err := c.Links.Add("docs/in", task.Path, "cites", "alice"); err != nil {
				t.Fatalf("Links.Add(in): %v", err)
			}

			got, err := c.Tasks.Links(task.Key, tt.dir)
			if err != nil {
				t.Fatalf("Links(%q, %q) error = %v", task.Key, tt.dir, err)
			}
			want := make([]sdk.Link, len(tt.want))
			for i, l := range tt.want {
				want[i] = sdk.Link{
					From:  strings.ReplaceAll(l.From, "{spec}", task.Path),
					To:    strings.ReplaceAll(l.To, "{spec}", task.Path),
					Label: l.Label,
				}
			}
			sortLinks(got)
			sortLinks(want)
			if len(got) != len(want) {
				t.Fatalf("Links(%q, %q) = %+v, want %+v", task.Key, tt.dir, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("Links(%q, %q)[%d] = %+v, want %+v", task.Key, tt.dir, i, got[i], want[i])
				}
			}
		})
	}
}

// TestTasksDeps builds the chain a <- b <- c and checks the dependency
// views, readiness and cycle rejection.
func TestTasksDeps(t *testing.T) {
	c := testHost(t)
	a := addTask(t, c, "A", spec, sdk.TaskAddOpts{Author: "alice"})
	b := addTask(t, c, "B", spec, sdk.TaskAddOpts{Author: "alice", DependsOn: a.Key})
	cc := addTask(t, c, "C", spec, sdk.TaskAddOpts{Author: "alice", DependsOn: b.Key})

	t.Run("Dep", func(t *testing.T) {
		tests := []struct {
			key  string
			want string // key of the dependency, "" for none
		}{
			{a.Key, ""},
			{b.Key, a.Key},
			{cc.Key, b.Key},
		}
		for _, tt := range tests {
			got, err := c.Tasks.Dep(tt.key)
			if err != nil {
				t.Fatalf("Dep(%q) error = %v", tt.key, err)
			}
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("Dep(%q) = %+v, want nil", tt.key, got)
			case tt.want != "" && (got == nil || got.Key != tt.want):
				t.Errorf("Dep(%q) = %+v, want %s", tt.key, got, tt.want)
			}
		}
	})

	t.Run("Dependents", func(t *testing.T) {
		tests := []struct {
			key  string
			want []string
		}{
			{a.Key, []string{b.Key}},
			{b.Key, []string{cc.Key}},
			{cc.Key, nil},
		}
		for _, tt := range tests {
			got, err := c.Tasks.Dependents(tt.key)
			if err != nil {
				t.Fatalf("Dependents(%q) error = %v", tt.key, err)
			}
			if keys := taskKeys(got); !slices.Equal(keys, tt.want) {
				t.Errorf("Dependents(%q) = %v, want %v", tt.key, keys, tt.want)
			}
		}
	})

	t.Run("Chain", func(t *testing.T) {
		tests := []struct {
			key  string
			want []string
		}{
			{a.Key, []string{a.Key}},
			{b.Key, []string{a.Key, b.Key}},
			{cc.Key, []string{a.Key, b.Key, cc.Key}},
		}
		for _, tt := range tests {
			got, err := c.Tasks.Chain(tt.key)
			if err != nil {
				t.Fatalf("Chain(%q) error = %v", tt.key, err)
			}
			if keys := taskKeys(got); !slices.Equal(keys, tt.want) {
				t.Errorf("Chain(%q) = %v, want %v", tt.key, keys, tt.want)
			}
		}
	})

	t.Run("Ready", func(t *testing.T) {
		if ok, err := c.Tasks.Ready(cc.Key); err != nil || ok {
			t.Errorf("Ready(c) with open dependencies = %v, %v, want false", ok, err)
		}
		moveTask(t, c, a.Key, "done")
		if ok, err := c.Tasks.Ready(b.Key); err != nil || !ok {
			t.Errorf("Ready(b) with a done = %v, %v, want true", ok, err)
		}
		if ok, err := c.Tasks.Ready(cc.Key); err != nil || ok {
			t.Errorf("Ready(c) with b open = %v, %v, want false", ok, err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		if err := c.Tasks.Set(a.Key, "alice", sdk.TaskSetOpts{DependsOn: ptr(cc.Key)}); !errors.Is(err, sdk.ErrCycle) {
			t.Errorf("Set(a depends on c) error = %v, want %v", err, sdk.ErrCycle)
		}
	})
}

func taskKeys(tt []*sdk.Task) []string {
	var keys []string
	for _, t := range tt {
		keys = append(keys, t.Key)
	}
	return keys
}

// TestTasksAddErrors covers the validation and lookup failures of Add
// and the branches a successful Add takes for dependencies and
// duplicate titles.
func TestTasksAddErrors(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		body    []byte
		opts    func(c sdk.Context, dep string) sdk.TaskAddOpts
		setup   func(t *testing.T, c sdk.Context)
		wantErr error
		check   func(t *testing.T, c sdk.Context, task *sdk.Task, dep string)
	}{
		{name: "empty title", title: "", wantErr: sdk.ErrMissingArg},
		{name: "null byte in title", title: "bad\x00title", wantErr: sdk.ErrInvalidArg},
		{name: "null byte in body", title: "ok", body: []byte("bad\x00body"), wantErr: sdk.ErrInvalidArg},
		{
			name:  "unknown column",
			title: "ok",
			opts: func(sdk.Context, string) sdk.TaskAddOpts {
				return sdk.TaskAddOpts{Status: "nowhere"}
			},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:  "missing spec path",
			title: "ok",
			opts: func(sdk.Context, string) sdk.TaskAddOpts {
				return sdk.TaskAddOpts{Path: "specs/missing"}
			},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:  "missing dependency",
			title: "ok",
			opts: func(sdk.Context, string) sdk.TaskAddOpts {
				return sdk.TaskAddOpts{DependsOn: "nosuchtask"}
			},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:  "dependency recorded",
			title: "ok",
			opts: func(_ sdk.Context, dep string) sdk.TaskAddOpts {
				return sdk.TaskAddOpts{DependsOn: dep}
			},
			check: func(t *testing.T, _ sdk.Context, task *sdk.Task, dep string) {
				if task.DependsOn != dep {
					t.Errorf("Add().DependsOn = %q, want %q", task.DependsOn, dep)
				}
			},
		},
		{
			name:  "duplicate title gets a numbered path",
			title: "Same title",
			setup: func(t *testing.T, c sdk.Context) {
				addTask(t, c, "Same title", nil, sdk.TaskAddOpts{Author: "alice"})
			},
			check: func(t *testing.T, _ sdk.Context, task *sdk.Task, _ string) {
				if task.Path != "tasks/same-title-2" {
					t.Errorf("Add().Path = %q, want %q", task.Path, "tasks/same-title-2")
				}
			},
		},
		{
			name:  "existing document path is not reused",
			title: "Taken",
			setup: func(t *testing.T, c sdk.Context) { writeDoc(t, c, "tasks/taken", "# Taken") },
			check: func(t *testing.T, _ sdk.Context, task *sdk.Task, _ string) {
				if task.Path != "tasks/taken-2" {
					t.Errorf("Add().Path = %q, want %q", task.Path, "tasks/taken-2")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			dep := addTask(t, c, "Dependency", nil, sdk.TaskAddOpts{Author: "alice"}).Key
			if tt.setup != nil {
				tt.setup(t, c)
			}
			opts := sdk.TaskAddOpts{}
			if tt.opts != nil {
				opts = tt.opts(c, dep)
			}
			opts.Author = "alice"

			task, err := c.Tasks.Add(tt.title, tt.body, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%q, %+v) error = %v, want %v", tt.title, opts, err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, c, task, dep)
			}
		})
	}
}

// TestTasksAddColumnErrors covers the failures of AddColumn.
func TestTasksAddColumnErrors(t *testing.T) {
	tests := []struct {
		name    string
		column  string
		after   string
		wantErr error
	}{
		{"duplicate", "review", "", sdk.ErrExists},
		{"after a missing column", "qa", "nowhere", sdk.ErrNotFound},
		{"null byte", "q\x00a", "", sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if err := c.Tasks.AddColumn(tt.column, tt.after, "alice"); !errors.Is(err, tt.wantErr) {
				t.Errorf("AddColumn(%q, %q) error = %v, want %v", tt.column, tt.after, err, tt.wantErr)
			}
		})
	}
}

// TestTaskErr covers the translation of every tasks sentinel.
func TestTaskErr(t *testing.T) {
	other := errors.New("disk on fire")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"task not found", tasks.ErrNotFound, sdk.ErrNotFound},
		{"document not found", documents.ErrNotFound, sdk.ErrNotFound},
		{"no spec", tasks.ErrNoSpec, sdk.ErrNoSpec},
		{"missing title", tasks.ErrMissingTitle, sdk.ErrMissingArg},
		{"invalid column", tasks.ErrInvalidCol, sdk.ErrInvalidArg},
		{"column not found", tasks.ErrColNotFound, sdk.ErrNotFound},
		{"column exists", tasks.ErrColExists, sdk.ErrExists},
		{"column not empty", tasks.ErrColNotEmpty, sdk.ErrInvalidArg},
		{"cycle", tasks.ErrCycle, sdk.ErrCycle},
		{"passthrough", other, other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := taskErr(tt.in)
			if !errors.Is(got, tt.want) {
				t.Errorf("taskErr(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.in != nil && !errors.Is(got, tt.in) {
				t.Errorf("taskErr(%v) = %v, want to wrap the original", tt.in, got)
			}
		})
	}
}
