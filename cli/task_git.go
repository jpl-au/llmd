// task_git.go provides git-aware task subcommands: start, diff, files.
//
// These commands bridge the task board with git, allowing users to
// associate a task with a branch and then view what changed.

package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskStart moves a task to in-progress and records the current git branch.
func taskStart(ctx sdk.Context, args []string) (sdk.Response, error) {
	column := "in-progress"
	var key string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task start: --column requires a value")
			}
			column = args[i]
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task start: %w: id", sdk.ErrMissingArg)
	}

	branch, err := gitBranch()
	if err != nil {
		return nil, fmt.Errorf("task start: %w", err)
	}

	if err := sdk.Tasks.Move(key, column, ctx.Author); err != nil {
		return nil, fmt.Errorf("task start: %w", err)
	}

	if err := sdk.Tasks.Set(key, ctx.Author, sdk.TaskSetOpts{Branch: &branch}); err != nil {
		return nil, fmt.Errorf("task start: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Started %s on branch %s", key, branch)), nil
}

// taskDiff shows the git diff for a task's branch against the default branch.
func taskDiff(_ sdk.Context, args []string) (sdk.Response, error) {
	var key, base string
	stat := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task diff: --base requires a value")
			}
			base = args[i]
		case "--stat":
			stat = true
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task diff: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(key)
	if err != nil {
		return nil, fmt.Errorf("task diff: %w", err)
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task diff: task has no branch — use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = gitDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task diff: %w — use --base to specify", err)
		}
	}

	output, err := gitDiff(base, t.Branch, stat)
	if err != nil {
		return nil, fmt.Errorf("task diff: %w", err)
	}

	if output == "" {
		return sdk.Text("No differences"), nil
	}

	if isTTY() {
		output = colourDiff(output)
	}

	return sdk.Text(output), nil
}

// taskFiles lists files changed on a task's branch.
func taskFiles(_ sdk.Context, args []string) (sdk.Response, error) {
	var key, base string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task files: --base requires a value")
			}
			base = args[i]
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task files: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(key)
	if err != nil {
		return nil, fmt.Errorf("task files: %w", err)
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task files: task has no branch — use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = gitDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task files: %w — use --base to specify", err)
		}
	}

	files, err := gitFiles(base, t.Branch)
	if err != nil {
		return nil, fmt.Errorf("task files: %w", err)
	}

	if len(files) == 0 {
		return sdk.Text("No changed files"), nil
	}

	return sdk.Result{Text: strings.Join(files, "\n"), Data: files}, nil
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
