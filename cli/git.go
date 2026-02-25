// git.go provides low-level git helpers for CLI commands.

package cli

import (
	"fmt"
	"os/exec"
	"strings"
)

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

// gitDefaultBranch returns the name of the default branch (main or master).
func gitDefaultBranch() (string, error) {
	for _, name := range []string{"main", "master"} {
		if err := exec.Command("git", "rev-parse", "--verify", name).Run(); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not detect default branch (tried main, master)")
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
	return strings.TrimRight(string(out), "\n"), nil
}

// gitFiles returns the list of files changed between base and branch.
func gitFiles(base, branch string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+"..."+branch).Output()
	if err != nil {
		return nil, gitError(err, branch)
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
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
