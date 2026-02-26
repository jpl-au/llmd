package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// testGitAndStore creates both a git repo and an llmd store, with the
// working directory set to the git repo. Returns the repo dir.
func testGitAndStore(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Init git repo
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	run("git", "commit", "--allow-empty", "-m", "initial")

	// Init llmd store in the same directory. llmd.Init creates the db
	// file; we close it and reopen via host.Open which wires up the
	// SDK globals (sdk.Documents, sdk.Tasks, etc.).
	dbPath := filepath.Join(dir, ".llmd", "llmd.db")
	store, err := llmd.Init(dbPath)
	if err != nil {
		t.Fatalf("llmd.Init: %v", err)
	}
	store.Close()

	h, err := host.Open(dbPath)
	if err != nil {
		t.Fatalf("host.Open: %v", err)
	}
	t.Cleanup(func() { h.Close() })

	return dir
}

func TestBranchSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Fix auth tokens", "fix-auth-tokens"},
		{"Add API endpoint", "add-api-endpoint"},
		{"Hello, World!", "hello-world"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPER CASE", "upper-case"},
		{"already-slugged", "already-slugged"},
		{"special!@#chars", "special-chars"},
	}
	for _, tt := range tests {
		got := branchSlug(tt.title)
		if got != tt.want {
			t.Errorf("branchSlug(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestTaskBranch(t *testing.T) {
	dir := testGitAndStore(t)

	// Create a task with a spec so it can move out of backlog
	task, err := sdk.Tasks.Add("Fix login flow", []byte("# Spec\n\nFix the login."), sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx := sdk.Context{Author: "alice"}
	resp, err := taskBranch(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskBranch: %v", err)
	}
	if resp == nil {
		t.Fatal("taskBranch returned nil response")
	}

	// Verify we're on the new branch
	branch, err := gitBranch()
	if err != nil {
		t.Fatalf("gitBranch: %v", err)
	}
	if branch != "task/fix-login-flow" {
		t.Errorf("branch = %q, want %q", branch, "task/fix-login-flow")
	}

	// Verify task was updated
	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Branch != "task/fix-login-flow" {
		t.Errorf("task branch = %q, want %q", updated.Branch, "task/fix-login-flow")
	}
	if updated.Status != "in-progress" {
		t.Errorf("task status = %q, want %q", updated.Status, "in-progress")
	}

	// Creating a branch for a task that already has one should error
	_, err = taskBranch(ctx, []string{task.Key})
	if err == nil {
		t.Error("expected error creating branch for task that already has one")
	}

	// Switch back to main for cleanup
	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	cmd.CombinedOutput()
}

func TestTaskBranchCustomName(t *testing.T) {
	dir := testGitAndStore(t)

	task, _ := sdk.Tasks.Add("Custom branch", []byte("# Spec\n\nCustom."), sdk.TaskAddOpts{Author: "alice"})

	ctx := sdk.Context{Author: "alice"}
	_, err := taskBranch(ctx, []string{task.Key, "--name", "feature/custom"})
	if err != nil {
		t.Fatalf("taskBranch: %v", err)
	}

	branch, _ := gitBranch()
	if branch != "feature/custom" {
		t.Errorf("branch = %q, want %q", branch, "feature/custom")
	}

	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	cmd.CombinedOutput()
}

func TestTaskFinish(t *testing.T) {
	dir := testGitAndStore(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}

	task, _ := sdk.Tasks.Add("Finish me", []byte("# Spec\n\nDo the thing."), sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Move(task.Key, "in-progress", "alice")

	// Create a branch with a commit
	run("git", "checkout", "-b", "feature/finish-test")
	branch := "feature/finish-test"
	sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Branch: &branch})
	os.WriteFile(filepath.Join(dir, "change.txt"), []byte("hello"), 0644)
	run("git", "add", "change.txt")
	run("git", "commit", "-m", "test commit")

	ctx := sdk.Context{Author: "alice"}
	resp, err := taskFinish(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskFinish: %v", err)
	}

	text := string(resp.(sdk.Text))
	if text == "" {
		t.Error("taskFinish returned empty response")
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "done" {
		t.Errorf("status = %q, want %q", updated.Status, "done")
	}
}

func TestTaskFinishWithoutGit(t *testing.T) {
	// No git repo — should still move the task to done
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	dbPath := filepath.Join(dir, ".llmd", "llmd.db")
	store, err := llmd.Init(dbPath)
	if err != nil {
		t.Fatalf("llmd.Init: %v", err)
	}
	store.Close()

	h, err := host.Open(dbPath)
	if err != nil {
		t.Fatalf("host.Open: %v", err)
	}
	t.Cleanup(func() { h.Close() })

	task, _ := sdk.Tasks.Add("No git", []byte("# Spec\n\nPlain task."), sdk.TaskAddOpts{Author: "alice"})
	sdk.Tasks.Move(task.Key, "in-progress", "alice")

	ctx := sdk.Context{Author: "alice"}
	_, err = taskFinish(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskFinish without git: %v", err)
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "done" {
		t.Errorf("status = %q, want %q", updated.Status, "done")
	}
}

func TestTaskCommits(t *testing.T) {
	dir := testGitAndStore(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}

	task, _ := sdk.Tasks.Add("Commits task", []byte("# Spec\n\nHas commits."), sdk.TaskAddOpts{Author: "alice"})
	run("git", "checkout", "-b", "feature/commits-test")
	branch := "feature/commits-test"
	sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Branch: &branch})

	run("git", "commit", "--allow-empty", "-m", "first")
	run("git", "commit", "--allow-empty", "-m", "second")

	ctx := sdk.Context{Author: "alice"}
	resp, err := taskCommits(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskCommits: %v", err)
	}

	result := resp.(sdk.Result)
	commits := result.Data.([]string)
	if len(commits) != 2 {
		t.Errorf("expected 2 commits, got %d", len(commits))
	}
}

func TestTaskForBranch(t *testing.T) {
	testGitAndStore(t)

	// Create a task linked to the current branch (main)
	task, _ := sdk.Tasks.Add("Main task", nil, sdk.TaskAddOpts{
		Author: "alice",
		Branch: "main",
	})

	found, err := taskForBranch()
	if err != nil {
		t.Fatalf("taskForBranch: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("found key = %q, want %q", found.Key, task.Key)
	}
}

func TestTaskForBranchNotFound(t *testing.T) {
	testGitAndStore(t)

	// No task linked to main
	_, err := taskForBranch()
	if err == nil {
		t.Fatal("expected error when no task linked to branch")
	}
}

func TestResolveTaskByKey(t *testing.T) {
	testGitAndStore(t)

	task, _ := sdk.Tasks.Add("By key", nil, sdk.TaskAddOpts{Author: "alice"})

	found, err := resolveTask("test", task.Key)
	if err != nil {
		t.Fatalf("resolveTask: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("found key = %q, want %q", found.Key, task.Key)
	}
}

func TestResolveTaskByBranch(t *testing.T) {
	testGitAndStore(t)

	task, _ := sdk.Tasks.Add("By branch", nil, sdk.TaskAddOpts{
		Author: "alice",
		Branch: "main",
	})

	// Empty key should auto-detect from current branch
	found, err := resolveTask("test", "")
	if err != nil {
		t.Fatalf("resolveTask: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("found key = %q, want %q", found.Key, task.Key)
	}
}
