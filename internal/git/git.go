// Package git provides git repository operations. The implementation
// shells out to the git CLI via os/exec. Platform-specific output
// handling (CRLF normalisation on Windows) is in build-tagged files.
package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Git implements [sdk.GitStore] by shelling out to the git CLI.
type Git struct{}

// New returns a Git instance.
func New() *Git {
	return &Git{}
}

// Available checks that git is installed and the working directory is
// inside a git repository. Uses --is-inside-work-tree and checks the
// exit code rather than parsing localised error messages.
func (g *Git) Available() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}
	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return fmt.Errorf("not in a git repository")
	}
	return nil
}

// Branch returns the current git branch name.
func (g *Git) Branch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("detecting git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("not on a branch (detached HEAD)")
	}
	return branch, nil
}

// DefaultBranch returns the name of the default branch. It tries the
// remote HEAD symbolic ref first (works when origin is configured),
// then falls back to checking whether main or master exist locally.
func (g *Git) DefaultBranch() (string, error) {
	// Try remote HEAD (e.g. "refs/remotes/origin/HEAD" → "origin/main").
	if out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		// "refs/remotes/origin/main" → "main"
		if parts := strings.SplitN(ref, "/", 4); len(parts) == 4 {
			return parts[3], nil
		}
	}

	// Fall back to checking local branches.
	for _, name := range []string{"main", "master"} {
		if err := exec.Command("git", "rev-parse", "--verify", name).Run(); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not detect default branch (tried origin/HEAD, main, master)")
}

// Diff runs git diff between base and branch using three-dot notation.
func (g *Git) Diff(base, branch string, opts sdk.DiffOpts) (string, error) {
	args := []string{"diff"}
	if opts.Stat {
		args = append(args, "--stat")
	}
	args = append(args, base+"..."+branch)

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", branchError(err, branch)
	}
	return strings.TrimRight(output(out), "\n"), nil
}

// Files returns the list of files changed between base and branch.
func (g *Git) Files(base, branch string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+"..."+branch).Output()
	if err != nil {
		return nil, branchError(err, branch)
	}
	text := strings.TrimSpace(output(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// Commits returns commit summaries on branch that are not on base.
// Each entry is a single-line "hash subject" string.
func (g *Git) Commits(base, branch string) ([]string, error) {
	out, err := exec.Command("git", "log", "--oneline", base+".."+branch).Output()
	if err != nil {
		return nil, branchError(err, branch)
	}
	text := strings.TrimSpace(output(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// CheckoutNew creates a new branch and switches to it.
func (g *Git) CheckoutNew(branch string) error {
	out, err := exec.Command("git", "checkout", "-b", branch).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "already exists") {
			return fmt.Errorf("branch %q already exists", branch)
		}
		return fmt.Errorf("git checkout -b: %s", msg)
	}
	return nil
}

// RevCount returns the number of commits ahead and behind between
// branch and base. Ahead is commits on branch not on base; behind is
// commits on base not on branch.
func (g *Git) RevCount(base, branch string) (ahead, behind int, err error) {
	out, err := exec.Command("git", "rev-list", "--count", "--left-right", base+"..."+branch).Output()
	if err != nil {
		return 0, 0, branchError(err, branch)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", string(out))
	}
	// left-right: left is base (behind), right is branch (ahead)
	behind, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing rev-list count: %w", err)
	}
	ahead, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing rev-list count: %w", err)
	}
	return ahead, behind, nil
}

// WorktreeAdd creates a new git worktree at path for the given branch.
func (g *Git) WorktreeAdd(path, branch string) error {
	out, err := exec.Command("git", "worktree", "add", path, branch).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git worktree add: %s", msg)
		}
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

// WorktreeRemove removes a git worktree at the given path.
func (g *Git) WorktreeRemove(path string) error {
	out, err := exec.Command("git", "worktree", "remove", "--force", path).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git worktree remove: %s", msg)
		}
		return fmt.Errorf("git worktree remove: %w", err)
	}
	return nil
}

// branchError extracts a useful message from a failed git command.
func branchError(err error, branch string) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if strings.Contains(stderr, "unknown revision") {
			return fmt.Errorf("branch %q not found in git", branch)
		}
		if stderr != "" {
			return fmt.Errorf("git: %s", stderr)
		}
	}
	return fmt.Errorf("running git: %w", err)
}
