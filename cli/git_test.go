package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// testGitRepo creates a temporary git repository with an initial commit
// on the "main" branch. It changes the working directory to the repo
// for the duration of the test and restores it on cleanup.
func testGitRepo(t *testing.T) string {
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

	// Create an initial commit so main exists as a real ref.
	run("git", "commit", "--allow-empty", "-m", "initial")

	return dir
}

func TestGitAvailable(t *testing.T) {
	// In a real git repo, gitAvailable should succeed.
	testGitRepo(t)
	if err := gitAvailable(); err != nil {
		t.Fatalf("expected no error in git repo, got: %v", err)
	}
}

func TestGitAvailableOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	err := gitAvailable()
	if err == nil {
		t.Fatal("expected error outside git repo")
	}
	if got := err.Error(); got != "not in a git repository" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestGitCommits(t *testing.T) {
	dir := testGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}

	// Create a feature branch with two commits.
	run("git", "checkout", "-b", "feature")
	run("git", "commit", "--allow-empty", "-m", "first change")
	run("git", "commit", "--allow-empty", "-m", "second change")

	commits, err := gitCommits("main", "feature")
	if err != nil {
		t.Fatalf("gitCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(commits), commits)
	}
}

func TestGitCommitsEmpty(t *testing.T) {
	testGitRepo(t)

	// No commits ahead of main on main itself.
	commits, err := gitCommits("main", "main")
	if err != nil {
		t.Fatalf("gitCommits: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}
}

func TestGitCheckoutNew(t *testing.T) {
	dir := testGitRepo(t)

	if err := gitCheckoutNew("task/my-feature"); err != nil {
		t.Fatalf("gitCheckoutNew: %v", err)
	}

	// Verify we're on the new branch.
	branch, err := gitBranch()
	if err != nil {
		t.Fatalf("gitBranch: %v", err)
	}
	if branch != "task/my-feature" {
		t.Fatalf("expected branch task/my-feature, got %s", branch)
	}

	// Creating the same branch again should fail.
	// Switch back to main first.
	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	cmd.CombinedOutput()

	err = gitCheckoutNew("task/my-feature")
	if err == nil {
		t.Fatal("expected error creating duplicate branch")
	}
}

func TestGitRevCount(t *testing.T) {
	dir := testGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}

	run("git", "checkout", "-b", "feature")
	run("git", "commit", "--allow-empty", "-m", "ahead 1")
	run("git", "commit", "--allow-empty", "-m", "ahead 2")

	ahead, behind, err := gitRevCount("main", "feature")
	if err != nil {
		t.Fatalf("gitRevCount: %v", err)
	}
	if ahead != 2 {
		t.Fatalf("expected ahead=2, got %d", ahead)
	}
	if behind != 0 {
		t.Fatalf("expected behind=0, got %d", behind)
	}

	// Add a commit to main so feature is behind.
	run("git", "checkout", "main")
	run("git", "commit", "--allow-empty", "-m", "main diverge")

	ahead, behind, err = gitRevCount("main", "feature")
	if err != nil {
		t.Fatalf("gitRevCount: %v", err)
	}
	if ahead != 2 {
		t.Fatalf("expected ahead=2, got %d", ahead)
	}
	if behind != 1 {
		t.Fatalf("expected behind=1, got %d", behind)
	}
}

func TestGitDefaultBranch(t *testing.T) {
	testGitRepo(t)

	branch, err := gitDefaultBranch()
	if err != nil {
		t.Fatalf("gitDefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected main, got %s", branch)
	}
}

func TestGitFiles(t *testing.T) {
	dir := testGitRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}

	run("git", "checkout", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644)
	run("git", "add", "new.txt")
	run("git", "commit", "-m", "add file")

	files, err := gitFiles("main", "feature")
	if err != nil {
		t.Fatalf("gitFiles: %v", err)
	}
	if len(files) != 1 || files[0] != "new.txt" {
		t.Fatalf("expected [new.txt], got %v", files)
	}
}
