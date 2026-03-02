// git.go provides low-level git helpers for CLI commands.

package cli

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// gitAvailable checks that git is installed and the working directory
// is inside a git repository. Returns nil if both conditions are met,
// or a descriptive error explaining what's missing.
func gitAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}
	out, err := exec.Command("git", "rev-parse", "--git-dir").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "not a git repository") {
			return fmt.Errorf("not in a git repository")
		}
		return fmt.Errorf("git: %s", msg)
	}
	return nil
}

// gitBranch returns the current git branch name.
func gitBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "not a git repository") {
				return "", fmt.Errorf("not in a git repository")
			}
		}
		return "", fmt.Errorf("detecting git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("not on a branch (detached HEAD)")
	}
	return branch, nil
}

// gitDefaultBranch returns the name of the default branch. It tries
// the remote HEAD symbolic ref first (works when origin is configured),
// then falls back to checking whether main or master exist locally.
func gitDefaultBranch() (string, error) {
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

// gitDiff runs git diff between base and branch using three-dot notation.
func gitDiff(base, branch string, stat bool) (string, error) {
	args := []string{"diff"}
	if stat {
		args = append(args, "--stat")
	}
	args = append(args, base+"..."+branch)

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", gitError(err, branch)
	}
	return strings.TrimRight(gitOutput(out), "\n"), nil
}

// gitFiles returns the list of files changed between base and branch.
func gitFiles(base, branch string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+"..."+branch).Output()
	if err != nil {
		return nil, gitError(err, branch)
	}
	text := strings.TrimSpace(gitOutput(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// gitCommits returns commit summaries on branch that are not on base.
// Each entry is a single-line "hash subject" string from git log --oneline.
func gitCommits(base, branch string) ([]string, error) {
	out, err := exec.Command("git", "log", "--oneline", base+".."+branch).Output()
	if err != nil {
		return nil, gitError(err, branch)
	}
	text := strings.TrimSpace(gitOutput(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// gitCheckoutNew creates a new branch and switches to it.
func gitCheckoutNew(branch string) error {
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

// gitRevCount returns the number of commits ahead and behind between
// branch and base. Ahead is commits on branch not on base; behind is
// commits on base not on branch.
func gitRevCount(base, branch string) (ahead, behind int, err error) {
	out, err := exec.Command("git", "rev-list", "--count", "--left-right", base+"..."+branch).Output()
	if err != nil {
		return 0, 0, gitError(err, branch)
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

// gitError extracts a useful message from a failed git command.
func gitError(err error, branch string) error {
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
