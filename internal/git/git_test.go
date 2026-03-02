package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// testRepo creates a temporary git repository with an initial commit
// on the "main" branch. It changes the working directory to the repo
// for the duration of the test and restores it on cleanup.
func testRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
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

func TestAvailable(t *testing.T) {
	testRepo(t)
	g := New()
	if err := g.Available(); err != nil {
		t.Fatalf("expected no error in git repo, got: %v", err)
	}
}

func TestAvailableOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	g := New()
	err := g.Available()
	if err == nil {
		t.Fatal("expected error outside git repo")
	}
	if got := err.Error(); got != "not in a git repository" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestBranch(t *testing.T) {
	testRepo(t)
	g := New()

	branch, err := g.Branch()
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected main, got %s", branch)
	}
}

func TestDefaultBranch(t *testing.T) {
	testRepo(t)
	g := New()

	branch, err := g.DefaultBranch()
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected main, got %s", branch)
	}
}

func TestCommits(t *testing.T) {
	dir := testRepo(t)
	g := New()

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

	commits, err := g.Commits("main", "feature")
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d: %v", len(commits), commits)
	}
}

func TestCommitsEmpty(t *testing.T) {
	testRepo(t)
	g := New()

	// No commits ahead of main on main itself.
	commits, err := g.Commits("main", "main")
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}
}

func TestCheckoutNew(t *testing.T) {
	dir := testRepo(t)
	g := New()

	if err := g.CheckoutNew("task/my-feature"); err != nil {
		t.Fatalf("CheckoutNew: %v", err)
	}

	// Verify we're on the new branch.
	branch, err := g.Branch()
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if branch != "task/my-feature" {
		t.Fatalf("expected branch task/my-feature, got %s", branch)
	}

	// Creating the same branch again should fail.
	// Switch back to main first.
	cmd := exec.Command("git", "checkout", "main")
	cmd.Dir = dir
	_, _ = cmd.CombinedOutput()

	err = g.CheckoutNew("task/my-feature")
	if err == nil {
		t.Fatal("expected error creating duplicate branch")
	}
}

func TestRevCount(t *testing.T) {
	dir := testRepo(t)
	g := New()

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

	ahead, behind, err := g.RevCount("main", "feature")
	if err != nil {
		t.Fatalf("RevCount: %v", err)
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

	ahead, behind, err = g.RevCount("main", "feature")
	if err != nil {
		t.Fatalf("RevCount: %v", err)
	}
	if ahead != 2 {
		t.Fatalf("expected ahead=2, got %d", ahead)
	}
	if behind != 1 {
		t.Fatalf("expected behind=1, got %d", behind)
	}
}

func TestFiles(t *testing.T) {
	dir := testRepo(t)
	g := New()

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
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("git", "add", "new.txt")
	run("git", "commit", "-m", "add file")

	files, err := g.Files("main", "feature")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) != 1 || files[0] != "new.txt" {
		t.Fatalf("expected [new.txt], got %v", files)
	}
}

func TestDiff(t *testing.T) {
	dir := testRepo(t)
	g := New()

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
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("git", "add", "new.txt")
	run("git", "commit", "-m", "add file")

	// Full diff should contain the file content.
	diff, err := g.Diff("main", "feature", sdk.DiffOpts{})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}

	// Stat diff should contain the filename.
	stat, err := g.Diff("main", "feature", sdk.DiffOpts{Stat: true})
	if err != nil {
		t.Fatalf("Diff(stat): %v", err)
	}
	if stat == "" {
		t.Fatal("expected non-empty stat diff")
	}
}
