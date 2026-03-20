package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

func testCtx(author string) sdk.Context {
	return sdk.Context{
		Context:   context.Background(),
		Author:    author,
		Documents: sdk.Documents,
		Tasks:     sdk.Tasks,
		Git:       sdk.Git,
	}
}

// testGitAndStore creates a disk-backed store and git repo in the same
// temp directory, with the working directory set to it.
func testGitAndStore(t *testing.T) string {
	t.Helper()

	dir := host.TestSetup(t, host.TestDisk)

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

	return dir
}

func TestTaskBranch(t *testing.T) {
	dir := testGitAndStore(t)

	// Create a task with a spec so it can move out of backlog
	task, err := sdk.Tasks.Add("Fix login flow", []byte("# Spec\n\nFix the login."), sdk.TaskAddOpts{Author: "alice"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx := testCtx("alice")
	resp, err := taskBranch(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskBranch: %v", err)
	}
	if resp == nil {
		t.Fatal("taskBranch returned nil response")
	}

	// Verify we're on the new branch
	branch, err := sdk.Git.Branch()
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
	_, _ = cmd.CombinedOutput()
}

func TestTaskBranchCustomName(t *testing.T) {
	dir := testGitAndStore(t)

	task, _ := sdk.Tasks.Add("Custom branch", []byte("# Spec\n\nCustom."), sdk.TaskAddOpts{Author: "alice"})

	ctx := testCtx("alice")
	_, err := taskBranch(ctx, []string{task.Key, "--name", "feature/custom"})
	if err != nil {
		t.Fatalf("taskBranch: %v", err)
	}

	branch, _ := sdk.Git.Branch()
	if branch != "feature/custom" {
		t.Errorf("branch = %q, want %q", branch, "feature/custom")
	}

	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	_, _ = cmd.CombinedOutput()
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
	if err := sdk.Tasks.Move(task.Key, "in-progress", "alice"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Create a branch with a commit
	run("git", "checkout", "-b", "feature/finish-test")
	branch := "feature/finish-test"
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Branch: &branch}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("git", "add", "change.txt")
	run("git", "commit", "-m", "test commit")

	ctx := testCtx("alice")
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
	// No git repo - should still move the task to done.
	host.TestSetup(t, host.TestDisk)

	task, _ := sdk.Tasks.Add("No git", []byte("# Spec\n\nPlain task."), sdk.TaskAddOpts{Author: "alice"})
	if err := sdk.Tasks.Move(task.Key, "in-progress", "alice"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	ctx := testCtx("alice")
	_, err := taskFinish(ctx, []string{task.Key})
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
	if err := sdk.Tasks.Set(task.Key, "alice", sdk.TaskSetOpts{Branch: &branch}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	run("git", "commit", "--allow-empty", "-m", "first")
	run("git", "commit", "--allow-empty", "-m", "second")

	ctx := testCtx("alice")
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

func TestResolveTaskByKey(t *testing.T) {
	testGitAndStore(t)

	task, _ := sdk.Tasks.Add("By key", nil, sdk.TaskAddOpts{Author: "alice"})

	found, err := resolveTask(testCtx("alice"), "test", task.Key)
	if err != nil {
		t.Fatalf("resolveTask: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("found key = %q, want %q", found.Key, task.Key)
	}
}

func TestTaskStartWithGit(t *testing.T) {
	testGitAndStore(t)

	task, _ := sdk.Tasks.Add("Start me", []byte("# Spec\n\nStart."), sdk.TaskAddOpts{Author: "alice"})

	ctx := testCtx("alice")
	resp, err := taskStart(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskStart: %v", err)
	}

	text := string(resp.(sdk.Text))
	if text == "" {
		t.Error("taskStart returned empty response")
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "in-progress" {
		t.Errorf("status = %q, want %q", updated.Status, "in-progress")
	}
	if updated.Branch != "main" {
		t.Errorf("branch = %q, want %q", updated.Branch, "main")
	}
}

func TestTaskStartWithoutGit(t *testing.T) {
	// No git repo - should still move the task to in-progress.
	host.TestSetup(t, host.TestDisk)

	task, _ := sdk.Tasks.Add("No git start", []byte("# Spec\n\nPlain."), sdk.TaskAddOpts{Author: "alice"})
	ctx := testCtx("alice")
	resp, err := taskStart(ctx, []string{task.Key})
	if err != nil {
		t.Fatalf("taskStart without git: %v", err)
	}

	text := string(resp.(sdk.Text))
	if text == "" {
		t.Error("taskStart returned empty response")
	}

	updated, _ := sdk.Tasks.Read(task.Key)
	if updated.Status != "in-progress" {
		t.Errorf("status = %q, want %q", updated.Status, "in-progress")
	}
	if updated.Branch != "" {
		t.Errorf("branch = %q, want empty", updated.Branch)
	}
}

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
		t.Errorf("expected 1 task on main, got %d", len(tasks))
	}
	if len(tasks) > 0 && tasks[0].Branch != "main" {
		t.Errorf("branch = %q, want %q", tasks[0].Branch, "main")
	}
}

func TestResolveTaskByBranch(t *testing.T) {
	testGitAndStore(t)

	task, _ := sdk.Tasks.Add("By branch", nil, sdk.TaskAddOpts{
		Author: "alice",
		Branch: "main",
	})

	// Empty key should auto-detect from current branch
	found, err := resolveTask(testCtx("alice"), "test", "")
	if err != nil {
		t.Fatalf("resolveTask: %v", err)
	}
	if found.Key != task.Key {
		t.Errorf("found key = %q, want %q", found.Key, task.Key)
	}
}
