package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// chdirTemp creates a temporary directory, changes the working
// directory to it for the duration of the test, and restores the
// original directory on cleanup.
func chdirTemp(t *testing.T) string {
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
	return dir
}

// gitRun runs a git command in dir and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
}

// testRepo creates a temporary git repository with an initial commit
// on the "main" branch. It changes the working directory to the repo
// for the duration of the test and restores it on cleanup.
func testRepo(t *testing.T) string {
	t.Helper()

	dir := chdirTemp(t)
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")

	// Create an initial commit so main exists as a real ref.
	gitRun(t, dir, "commit", "--allow-empty", "-m", "initial")

	return dir
}

// featureWithFile creates a "feature" branch off main with one commit
// that adds new.txt. Used by the Files and Diff tables.
func featureWithFile(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRun(t, dir, "add", "new.txt")
	gitRun(t, dir, "commit", "-m", "add file")
}

// featureWithCommits creates a "feature" branch off main with two
// empty commits. Used by the Commits and RevCount tables.
func featureWithCommits(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "checkout", "-b", "feature")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "first change")
	gitRun(t, dir, "commit", "--allow-empty", "-m", "second change")
}

func TestAvailable(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
	}{
		{"inside repo", testRepo, ""},
		{"outside repo", chdirTemp, "this is not a git repository."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			err := New(0).Available()
			var got string
			if err != nil {
				got = err.Error()
			}
			if got != tt.wantErr {
				t.Errorf("Available() error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestBranch(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		want    string
		wantErr bool
	}{
		{name: "main", want: "main"},
		{
			name:  "feature branch",
			setup: func(t *testing.T, dir string) { gitRun(t, dir, "checkout", "-b", "feature") },
			want:  "feature",
		},
		{
			name:    "detached head",
			setup:   func(t *testing.T, dir string) { gitRun(t, dir, "checkout", "--detach") },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New(0).Branch()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Branch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Branch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		want    string
		wantErr bool
	}{
		{name: "main", want: "main"},
		{
			name:  "master fallback",
			setup: func(t *testing.T, dir string) { gitRun(t, dir, "branch", "-m", "main", "master") },
			want:  "master",
		},
		{
			// origin/HEAD wins over local main when configured.
			name: "origin head",
			setup: func(t *testing.T, dir string) {
				gitRun(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
			},
			want: "develop",
		},
		{
			name:    "no default",
			setup:   func(t *testing.T, dir string) { gitRun(t, dir, "branch", "-m", "main", "trunk") },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New(0).DefaultBranch()
			if (err != nil) != tt.wantErr {
				t.Fatalf("DefaultBranch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("DefaultBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommits(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		base    string
		branch  string
		want    int
		wantErr string
	}{
		{name: "two ahead", setup: featureWithCommits, base: "main", branch: "feature", want: 2},
		// No commits ahead of main on main itself.
		{name: "same branch", base: "main", branch: "main", want: 0},
		{name: "unknown branch", base: "main", branch: "nope", wantErr: `git has no branch nope.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New(0).Commits(tt.base, tt.branch)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Commits(%q, %q) error = %v, want %q", tt.base, tt.branch, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Commits(%q, %q) error = %v", tt.base, tt.branch, err)
			}
			if len(got) != tt.want {
				t.Errorf("Commits(%q, %q) = %v (%d commits), want %d", tt.base, tt.branch, got, len(got), tt.want)
			}
		})
	}
}

func TestCheckoutNew(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string)
		branch  string
		wantErr string
	}{
		{name: "new branch", branch: "task/my-feature"},
		{
			// Creating the same branch again should fail.
			name:    "duplicate branch",
			setup:   func(t *testing.T, dir string) { gitRun(t, dir, "branch", "task/my-feature") },
			branch:  "task/my-feature",
			wantErr: `the branch task/my-feature already exists.`,
		},
		{name: "invalid name", branch: "bad..name", wantErr: "git checkout -b: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			g := New(0)
			err := g.CheckoutNew(tt.branch)
			if tt.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
					t.Fatalf("CheckoutNew(%q) error = %v, want prefix %q", tt.branch, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckoutNew(%q) error = %v", tt.branch, err)
			}
			// Verify we're on the new branch.
			got, err := g.Branch()
			if err != nil {
				t.Fatalf("Branch() error = %v", err)
			}
			if got != tt.branch {
				t.Errorf("Branch() after CheckoutNew(%q) = %q, want %q", tt.branch, got, tt.branch)
			}
		})
	}
}

func TestRevCount(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string)
		branch     string
		wantAhead  int
		wantBehind int
		wantErr    string
	}{
		{name: "same branch", branch: "main"},
		{name: "ahead only", setup: featureWithCommits, branch: "feature", wantAhead: 2},
		{
			// Add a commit to main so feature is also behind.
			name: "ahead and behind",
			setup: func(t *testing.T, dir string) {
				featureWithCommits(t, dir)
				gitRun(t, dir, "checkout", "main")
				gitRun(t, dir, "commit", "--allow-empty", "-m", "main diverge")
			},
			branch:     "feature",
			wantAhead:  2,
			wantBehind: 1,
		},
		{name: "unknown branch", branch: "nope", wantErr: `git has no branch nope.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			ahead, behind, err := New(0).RevCount("main", tt.branch)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("RevCount(main, %q) error = %v, want %q", tt.branch, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RevCount(main, %q) error = %v", tt.branch, err)
			}
			if ahead != tt.wantAhead || behind != tt.wantBehind {
				t.Errorf("RevCount(main, %q) = ahead %d, behind %d, want ahead %d, behind %d",
					tt.branch, ahead, behind, tt.wantAhead, tt.wantBehind)
			}
		})
	}
}

func TestFiles(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, dir string)
		branch string
		want   []string
	}{
		{name: "one file", setup: featureWithFile, branch: "feature", want: []string{"new.txt"}},
		{name: "no changes", branch: "main", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New(0).Files("main", tt.branch)
			if err != nil {
				t.Fatalf("Files(main, %q) error = %v", tt.branch, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Files(main, %q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, dir string)
		branch       string
		opts         sdk.DiffOpts
		wantContains string
		wantEmpty    bool
	}{
		// Full diff should contain the file content.
		{name: "full", setup: featureWithFile, branch: "feature", wantContains: "+hello"},
		// Stat diff should contain the filename.
		{name: "stat", setup: featureWithFile, branch: "feature", opts: sdk.DiffOpts{Stat: true}, wantContains: "new.txt"},
		{name: "no changes", branch: "main", wantEmpty: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New(0).Diff("main", tt.branch, tt.opts)
			if err != nil {
				t.Fatalf("Diff(main, %q, %+v) error = %v", tt.branch, tt.opts, err)
			}
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("Diff(main, %q, %+v) = %q, want empty", tt.branch, tt.opts, got)
				}
				return
			}
			if got == "" {
				t.Fatalf("Diff(main, %q, %+v) = empty, want non-empty", tt.branch, tt.opts)
			}
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("Diff(main, %q, %+v) = %q, want to contain %q", tt.branch, tt.opts, got, tt.wantContains)
			}
		})
	}
}

// gitOut runs a git command in dir and returns its trimmed output,
// failing the test on error.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

// worktreeCount returns how many working trees git records for the
// repository at dir, including the main one.
func worktreeCount(t *testing.T, dir string) int {
	t.Helper()
	var n int
	for line := range strings.Lines(gitOut(t, dir, "worktree", "list", "--porcelain")) {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}

// occupy creates path as a non-empty directory. git refuses to place a
// worktree in it (an empty directory would be accepted).
func occupy(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// checkWorktreeError asserts err carries the command prefix and git's
// reason: a failed worktree command reports git's own message after
// the prefix.
func checkWorktreeError(t *testing.T, call string, err error, prefix, reason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s error = nil, want prefix %q containing %q", call, prefix, reason)
	}
	if !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), reason) {
		t.Fatalf("%s error = %q, want prefix %q containing %q", call, err, prefix, reason)
	}
}

func TestWorktreeCreate(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir, path string)
		branch     string
		wantReason string // substring of git's message after "git worktree create: "
	}{
		{name: "new branch", branch: "task/my-feature"},
		{
			name:       "existing path",
			setup:      func(t *testing.T, _, path string) { occupy(t, path) },
			branch:     "task/my-feature",
			wantReason: "already exists",
		},
		{
			name:       "duplicate branch",
			setup:      func(t *testing.T, dir, _ string) { gitRun(t, dir, "branch", "task/my-feature") },
			branch:     "task/my-feature",
			wantReason: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			path := filepath.Join(t.TempDir(), "wt")
			if tt.setup != nil {
				tt.setup(t, dir, path)
			}
			g := New(0)
			err := g.WorktreeCreate(path, tt.branch)
			if tt.wantReason != "" {
				checkWorktreeError(t, "WorktreeCreate", err, "git worktree create: ", tt.wantReason)
				return
			}
			if err != nil {
				t.Fatalf("WorktreeCreate(%q, %q) error = %v", path, tt.branch, err)
			}
			// The new branch is checked out in the worktree and the main
			// working directory is left on main.
			if got := gitOut(t, path, "branch", "--show-current"); got != tt.branch {
				t.Errorf("branch in worktree = %q, want %q", got, tt.branch)
			}
			if got, err := g.Branch(); err != nil || got != "main" {
				t.Errorf("Branch() in main repo = %q, %v, want %q", got, err, "main")
			}
			if got := worktreeCount(t, dir); got != 2 {
				t.Errorf("worktree count after WorktreeCreate = %d, want 2", got)
			}
		})
	}
}

func TestWorktreeAdd(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir, path string)
		branch     string
		wantReason string // substring of git's message after "git worktree add: "
	}{
		{
			name:   "existing branch",
			setup:  func(t *testing.T, dir, _ string) { gitRun(t, dir, "branch", "feature") },
			branch: "feature",
		},
		{
			name: "existing path",
			setup: func(t *testing.T, dir, path string) {
				gitRun(t, dir, "branch", "feature")
				occupy(t, path)
			},
			branch:     "feature",
			wantReason: "already exists",
		},
		{name: "unknown branch", branch: "nope", wantReason: "invalid reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			path := filepath.Join(t.TempDir(), "wt")
			if tt.setup != nil {
				tt.setup(t, dir, path)
			}
			g := New(0)
			err := g.WorktreeAdd(path, tt.branch)
			if tt.wantReason != "" {
				checkWorktreeError(t, "WorktreeAdd", err, "git worktree add: ", tt.wantReason)
				return
			}
			if err != nil {
				t.Fatalf("WorktreeAdd(%q, %q) error = %v", path, tt.branch, err)
			}
			if got := gitOut(t, path, "branch", "--show-current"); got != tt.branch {
				t.Errorf("branch in worktree = %q, want %q", got, tt.branch)
			}
			if got, err := g.Branch(); err != nil || got != "main" {
				t.Errorf("Branch() in main repo = %q, %v, want %q", got, err, "main")
			}
			if got := worktreeCount(t, dir); got != 2 {
				t.Errorf("worktree count after WorktreeAdd = %d, want 2", got)
			}
		})
	}
}

func TestWorktreeRemove(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir, path string)
		wantReason string // substring of git's message after "git worktree remove: "
		wantExists bool   // path still present afterwards
	}{
		{
			name: "removes worktree",
			setup: func(t *testing.T, _, path string) {
				if err := New(0).WorktreeCreate(path, "feature"); err != nil {
					t.Fatalf("WorktreeCreate(%q, feature) error = %v", path, err)
				}
			},
		},
		{
			// A plain directory is refused and left in place.
			name:       "not a worktree",
			setup:      func(t *testing.T, _, path string) { occupy(t, path) },
			wantReason: "is not a working tree",
			wantExists: true,
		},
		{name: "missing path", wantReason: "is not a working tree"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			path := filepath.Join(t.TempDir(), "wt")
			if tt.setup != nil {
				tt.setup(t, dir, path)
			}
			err := New(0).WorktreeRemove(path)
			if tt.wantReason != "" {
				checkWorktreeError(t, "WorktreeRemove", err, "git worktree remove: ", tt.wantReason)
			} else if err != nil {
				t.Fatalf("WorktreeRemove(%q) error = %v", path, err)
			}
			if _, err := os.Stat(path); (err == nil) != tt.wantExists {
				t.Errorf("Stat(%q) error = %v, want exists %v", path, err, tt.wantExists)
			}
			if got := worktreeCount(t, dir); got != 1 {
				t.Errorf("worktree count after WorktreeRemove = %d, want 1", got)
			}
			if tt.wantReason == "" {
				// Removing the worktree keeps the branch it had checked out.
				if got := gitOut(t, dir, "branch", "--list", "feature"); got == "" {
					t.Errorf("branch feature missing after WorktreeRemove, want kept")
				}
			}
		})
	}
}

// TestBranchError drives branchError with real git failures so the
// *exec.ExitError carries the stderr git actually produces: an unknown
// revision maps to the branch-not-found message, any other stderr is
// reported verbatim, and an exit with no stderr or a non-exit error
// falls through to the wrapped form.
func TestBranchError(t *testing.T) {
	sentinel := errors.New("git vanished")

	tests := []struct {
		name     string
		err      func(t *testing.T, dir string) error
		branch   string
		want     string
		wantWrap error // expected to be found with errors.Is, if set
	}{
		{
			name: "unknown revision",
			err: func(t *testing.T, dir string) error {
				_, err := exec.Command("git", "rev-list", "--count", "main...nope").Output()
				return err
			},
			branch: "nope",
			want:   `git has no branch nope.`,
		},
		{
			name: "other stderr",
			err: func(t *testing.T, dir string) error {
				_, err := exec.Command("git", "cat-file", "-t", "nope").Output()
				return err
			},
			branch: "nope",
			want:   "git: fatal: Not a valid object name nope",
		},
		{
			// merge-base --is-ancestor exits 1 silently when the answer
			// is no, so the ExitError has no stderr to report.
			name: "exit without stderr",
			err: func(t *testing.T, dir string) error {
				featureWithCommits(t, dir)
				_, err := exec.Command("git", "merge-base", "--is-ancestor", "feature", "main").Output()
				return err
			},
			branch: "feature",
			want:   "running git: exit status 1",
		},
		{
			name:     "not an exit error",
			err:      func(t *testing.T, dir string) error { return sentinel },
			branch:   "feature",
			want:     "running git: git vanished",
			wantWrap: sentinel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			in := tt.err(t, dir)
			if in == nil {
				t.Fatal("setup produced no error, want a git failure")
			}
			got := branchError(in, tt.branch)
			if got == nil || got.Error() != tt.want {
				t.Errorf("branchError(%v, %q) = %v, want %q", in, tt.branch, got, tt.want)
			}
			if tt.wantWrap != nil && !errors.Is(got, tt.wantWrap) {
				t.Errorf("errors.Is(branchError(%v, %q), %v) = false, want true", in, tt.branch, tt.wantWrap)
			}
		})
	}
}

// TestOptionLikeRefs checks that a ref or path starting with "-" is
// refused before git runs: "--output=<file>" as a base once made git
// diff write that file, and a URL query could supply it.
func TestOptionLikeRefs(t *testing.T) {
	dir := testRepo(t)
	written := filepath.Join(t.TempDir(), "written")
	g := New(0)

	calls := []struct {
		name string
		call func() error
	}{
		{"Diff base", func() error { _, err := g.Diff("--output="+written, "main", sdk.DiffOpts{}); return err }},
		{"Diff branch", func() error { _, err := g.Diff("main", "--output="+written, sdk.DiffOpts{}); return err }},
		{"Files", func() error { _, err := g.Files("--output="+written, "main"); return err }},
		{"Commits", func() error { _, err := g.Commits("--output="+written, "main"); return err }},
		{"RevCount", func() error { _, _, err := g.RevCount("--output="+written, "main"); return err }},
		{"CheckoutNew", func() error { return g.CheckoutNew("--orphan") }},
		{"WorktreeAdd path", func() error { return g.WorktreeAdd("--detach", "main") }},
		{"WorktreeCreate branch", func() error { return g.WorktreeCreate(filepath.Join(dir, "wt"), "-x") }},
		{"WorktreeRemove", func() error { return g.WorktreeRemove("--force") }},
	}
	for _, tt := range calls {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, sdk.ErrInvalidArg) {
				t.Errorf("%s error = %v, want %v", tt.name, err, sdk.ErrInvalidArg)
			}
			matches, _ := filepath.Glob(written + "*")
			if len(matches) > 0 {
				t.Errorf("%s let git write %v", tt.name, matches)
			}
		})
	}
}
