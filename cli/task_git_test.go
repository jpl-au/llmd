package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

func testCtx(t *testing.T, author string) sdk.Context {
	t.Helper()
	return sdk.Context{
		Context:   t.Context(),
		Author:    author,
		Documents: sdk.Documents,
		Tasks:     sdk.Tasks,
		Git:       sdk.Git,
	}
}

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
}

// testGitAndStore creates a disk-backed store and git repo in the same
// temp directory, with the working directory set to it.
func testGitAndStore(t *testing.T) string {
	t.Helper()

	dir := host.TestSetup(t, host.TestDisk)
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")

	return dir
}

// addTask creates a task with a spec body so it can move out of backlog.
func addTask(t *testing.T, title string) *sdk.Task {
	t.Helper()
	task, err := sdk.Tasks.Add(title, []byte("# Spec\n\n"+title+"."), sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Add(%q): %v", title, err)
	}
	return task
}

// TestTaskBranch covers branch naming (title slug vs --name) and the
// rejection of a task that already has a branch. Each case gets a
// fresh repo and store.
func TestTaskBranch(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		args       []string // extra args after the task key
		setup      func(t *testing.T, ctx sdk.Context, key string)
		wantBranch string
		wantErr    bool
	}{
		{name: "slug from title", title: "Fix login flow", wantBranch: "task/fix-login-flow"},
		{name: "custom name", title: "Custom branch", args: []string{"--name", "feature/custom"}, wantBranch: "feature/custom"},
		{name: "slug collapses punctuation", title: "Add  OAuth2 (Google) support!", wantBranch: "task/add-oauth2-google-support"},
		{name: "slug lowercases", title: "UPPER Case 123", wantBranch: "task/upper-case-123"},
		{
			// Creating a branch for a task that already has one should error.
			name:  "already has branch",
			title: "Fix login flow",
			setup: func(t *testing.T, ctx sdk.Context, key string) {
				if _, err := taskBranch(ctx, []string{key}); err != nil {
					t.Fatalf("taskBranch (setup): %v", err)
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGitAndStore(t)
			task := addTask(t, tt.title)
			ctx := testCtx(t, "alice")
			if tt.setup != nil {
				tt.setup(t, ctx, task.Key)
			}

			args := append([]string{task.Key}, tt.args...)
			resp, err := taskBranch(ctx, args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("taskBranch(%v) error = %v, wantErr %v", args, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if resp == nil {
				t.Fatal("taskBranch returned nil response")
			}

			// Verify we're on the new branch.
			branch, err := sdk.Git.Branch()
			if err != nil {
				t.Fatalf("Git.Branch: %v", err)
			}
			if branch != tt.wantBranch {
				t.Errorf("git branch = %q, want %q", branch, tt.wantBranch)
			}

			// Verify task was updated.
			updated, err := sdk.Tasks.Read(task.Key)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if updated.Branch != tt.wantBranch {
				t.Errorf("task branch = %q, want %q", updated.Branch, tt.wantBranch)
			}
			if updated.Status != "in-progress" {
				t.Errorf("task status = %q, want %q", updated.Status, "in-progress")
			}
		})
	}
}

// TestTaskFinish covers finishing with a git branch (summary counts
// reported) and without any git repo (task still moves to done).
func TestTaskFinish(t *testing.T) {
	tests := []struct {
		name         string
		git          bool
		setup        func(t *testing.T, dir, key string)
		wantContains []string
	}{
		{
			name: "with git branch and commit",
			git:  true,
			setup: func(t *testing.T, dir, key string) {
				runGit(t, dir, "checkout", "-b", "feature/finish-test")
				branch := "feature/finish-test"
				if err := sdk.Tasks.Set(key, "alice", sdk.TaskSetOpts{Branch: &branch}); err != nil {
					t.Fatalf("Set: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("hello"), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				runGit(t, dir, "add", "change.txt")
				runGit(t, dir, "commit", "-m", "test commit")
			},
			wantContains: []string{"Finished ", "1 file(s) changed", "1 commit(s)"},
		},
		{
			// No git repo - should still move the task to done.
			name:         "without git",
			wantContains: []string{"Finished "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string
			if tt.git {
				dir = testGitAndStore(t)
			} else {
				dir = host.TestSetup(t, host.TestDisk)
			}
			task := addTask(t, "Finish me")
			if err := sdk.Tasks.Move(task.Key, "in-progress", "alice"); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, dir, task.Key)
			}

			resp, err := taskFinish(testCtx(t, "alice"), []string{task.Key})
			if err != nil {
				t.Fatalf("taskFinish(%q) error = %v", task.Key, err)
			}
			text := string(resp.(sdk.Text))
			if text == "" {
				t.Error("taskFinish returned empty response")
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("taskFinish(%q) = %q, want to contain %q", task.Key, text, want)
				}
			}

			updated, err := sdk.Tasks.Read(task.Key)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if updated.Status != "done" {
				t.Errorf("status = %q, want %q", updated.Status, "done")
			}
		})
	}
}

// TestTaskCommits is a scenario test: a branch with two commits is
// linked to a task and the command lists both.
func TestTaskCommits(t *testing.T) {
	dir := testGitAndStore(t)

	task := addTask(t, "Commits task")
	runGit(t, dir, "checkout", "-b", "feature/commits-test")
	branch := "feature/commits-test"
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Branch: &branch}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	runGit(t, dir, "commit", "--allow-empty", "-m", "first")
	runGit(t, dir, "commit", "--allow-empty", "-m", "second")

	resp, err := taskCommits(testCtx(t, "alice"), []string{task.Key})
	if err != nil {
		t.Fatalf("taskCommits: %v", err)
	}

	result := resp.(sdk.Result)
	commits := result.Data.([]string)
	if len(commits) != 2 {
		t.Errorf("taskCommits(%q) = %d commits, want 2", task.Key, len(commits))
	}
}

// TestResolveTask covers lookup by explicit key and auto-detection
// from the current git branch, plus the failure of each.
func TestResolveTask(t *testing.T) {
	tests := []struct {
		name    string
		branch  string // branch recorded on the task when added
		byKey   bool   // pass the task key; otherwise pass "" to auto-detect
		key     string // explicit key override (unknown key case)
		wantErr bool
	}{
		{name: "by key", byKey: true},
		// Empty key should auto-detect from current branch.
		{name: "by branch", branch: "main"},
		{name: "unknown key", byKey: true, key: "zzzzzzzzz", wantErr: true},
		{name: "no task on branch", branch: "feature/other", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGitAndStore(t)

			task, err := sdk.Tasks.Add("Resolve me", nil, sdk.TaskAddOpts{Author: "alice", Branch: tt.branch})
			if err != nil {
				t.Fatalf("Add: %v", err)
			}

			key := ""
			if tt.byKey {
				key = task.Key
			}
			if tt.key != "" {
				key = tt.key
			}

			found, err := resolveTask(testCtx(t, "alice"), "test", key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveTask(%q) error = %v, wantErr %v", key, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if found.Key != task.Key {
				t.Errorf("resolveTask(%q) key = %q, want %q", key, found.Key, task.Key)
			}
		})
	}
}

// TestTaskStart covers starting with git (current branch recorded),
// without git (branch left empty), and the missing-id error.
func TestTaskStart(t *testing.T) {
	tests := []struct {
		name       string
		git        bool
		noKey      bool
		wantBranch string
		wantErr    bool
	}{
		{name: "with git", git: true, wantBranch: "main"},
		// No git repo - should still move the task to in-progress.
		{name: "without git", wantBranch: ""},
		{name: "missing id", git: true, noKey: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.git {
				testGitAndStore(t)
			} else {
				host.TestSetup(t, host.TestDisk)
			}
			task := addTask(t, "Start me")

			var args []string
			if !tt.noKey {
				args = []string{task.Key}
			}
			resp, err := taskStart(testCtx(t, "alice"), args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("taskStart(%v) error = %v, wantErr %v", args, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			text := string(resp.(sdk.Text))
			if text == "" {
				t.Error("taskStart returned empty response")
			}

			updated, err := sdk.Tasks.Read(task.Key)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if updated.Status != "in-progress" {
				t.Errorf("status = %q, want %q", updated.Status, "in-progress")
			}
			if updated.Branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", updated.Branch, tt.wantBranch)
			}
		})
	}
}

// TestListByBranch is a scenario test for the SDK's branch filter,
// which backs auto-detection in the git-aware commands.
func TestListByBranch(t *testing.T) {
	testGitAndStore(t)

	if _, err := sdk.Tasks.Add("On main", nil, sdk.TaskAddOpts{Author: "alice", Branch: "main"}); err != nil {
		t.Fatalf("Add main: %v", err)
	}
	if _, err := sdk.Tasks.Add("On feature", nil, sdk.TaskAddOpts{Author: "alice", Branch: "feature/x"}); err != nil {
		t.Fatalf("Add feature: %v", err)
	}
	if _, err := sdk.Tasks.Add("No branch", nil, sdk.TaskAddOpts{Author: "alice"}); err != nil {
		t.Fatalf("Add no branch: %v", err)
	}

	tasks, err := sdk.Tasks.List(sdk.TaskListOpts{Branch: "main"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("List(Branch: main) = %d tasks, want 1", len(tasks))
	}
	if len(tasks) > 0 && tasks[0].Branch != "main" {
		t.Errorf("branch = %q, want %q", tasks[0].Branch, "main")
	}
}
