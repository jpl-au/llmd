package git

import (
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
		{"outside repo", chdirTemp, "not in a git repository"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			err := New().Available()
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
			got, err := New().Branch()
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
			got, err := New().DefaultBranch()
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
		{name: "unknown branch", base: "main", branch: "nope", wantErr: `branch "nope" not found in git`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			got, err := New().Commits(tt.base, tt.branch)
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
			wantErr: `branch "task/my-feature" already exists`,
		},
		{name: "invalid name", branch: "bad..name", wantErr: "git checkout -b: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			g := New()
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
		wantAhead  int
		wantBehind int
	}{
		{name: "same branch"},
		{name: "ahead only", setup: featureWithCommits, wantAhead: 2},
		{
			// Add a commit to main so feature is also behind.
			name: "ahead and behind",
			setup: func(t *testing.T, dir string) {
				featureWithCommits(t, dir)
				gitRun(t, dir, "checkout", "main")
				gitRun(t, dir, "commit", "--allow-empty", "-m", "main diverge")
			},
			wantAhead:  2,
			wantBehind: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t)
			branch := "main"
			if tt.setup != nil {
				tt.setup(t, dir)
				branch = "feature"
			}
			ahead, behind, err := New().RevCount("main", branch)
			if err != nil {
				t.Fatalf("RevCount(main, %q) error = %v", branch, err)
			}
			if ahead != tt.wantAhead || behind != tt.wantBehind {
				t.Errorf("RevCount(main, %q) = ahead %d, behind %d, want ahead %d, behind %d",
					branch, ahead, behind, tt.wantAhead, tt.wantBehind)
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
			got, err := New().Files("main", tt.branch)
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
			got, err := New().Diff("main", tt.branch, tt.opts)
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
