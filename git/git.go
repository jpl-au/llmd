// Package git provides git repository operations. The implementation
// shells out to the git CLI via os/exec. Platform-specific output
// handling (CRLF normalisation on Windows) is in build-tagged files.
//
// Refs and paths reach git from flags and URLs, so every one is checked
// before git sees it: a value starting with "-" would be read as an
// option (--output=<file> makes git diff write a file). Revision
// arguments also follow --end-of-options. Each command runs with a
// timeout and with terminal prompts off, so a git waiting for
// credentials or a lock fails instead of hanging the caller.
package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

// DefaultTimeout is how long one git command may run.
const DefaultTimeout = 2 * time.Minute

// Git implements [sdk.GitStore] by shelling out to the git CLI.
type Git struct {
	timeout time.Duration
}

// New returns a Git instance whose commands time out after timeout;
// zero or less means DefaultTimeout.
func New(timeout time.Duration) *Git {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Git{timeout: timeout}
}

// command builds a git command that times out and never prompts. The
// returned cancel must be called once the command has finished.
func (g *Git) command(args ...string) (*exec.Cmd, context.CancelFunc) {
	timeout := g.timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd, cancel
}

// output runs git and returns its standard output.
func (g *Git) output(args ...string) ([]byte, error) {
	cmd, cancel := g.command(args...)
	defer cancel()
	return cmd.Output()
}

// combined runs git and returns its standard output and error together.
func (g *Git) combined(args ...string) ([]byte, error) {
	cmd, cancel := g.command(args...)
	defer cancel()
	return cmd.CombinedOutput()
}

// checkRefs refuses refs git would read as options.
func checkRefs(refs ...string) error {
	for _, r := range refs {
		if r == "" || strings.HasPrefix(r, "-") {
			return sdk.Errorf(sdk.ErrInvalidArg, "%q is not a valid git ref.", r)
		}
	}
	return nil
}

// checkPath refuses a worktree path git would read as an option.
func checkPath(p string) error {
	if p == "" || strings.HasPrefix(p, "-") {
		return sdk.Errorf(sdk.ErrInvalidArg, "%q is not a valid worktree path.", p)
	}
	return nil
}

// Available checks that git is installed and the working directory is
// inside a git repository. Uses --is-inside-work-tree and checks the
// exit code rather than parsing localised error messages.
func (g *Git) Available() error {
	if _, err := exec.LookPath("git"); err != nil {
		return sdk.Errorf(sdk.ErrUnavailable, "git is not installed.")
	}
	if _, err := g.output("rev-parse", "--is-inside-work-tree"); err != nil {
		return sdk.Errorf(sdk.ErrUnavailable, "this is not a git repository.")
	}
	return nil
}

// Branch returns the current git branch name.
func (g *Git) Branch() (string, error) {
	out, err := g.output("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("detecting git branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", sdk.Errorf(sdk.ErrUnavailable, "the repository is not on a branch (detached HEAD).")
	}
	return branch, nil
}

// DefaultBranch returns the name of the default branch. It tries the
// remote HEAD symbolic ref first (works when origin is configured),
// then falls back to checking whether main or master exist locally.
func (g *Git) DefaultBranch() (string, error) {
	// Try remote HEAD (e.g. "refs/remotes/origin/HEAD" → "origin/main").
	if out, err := g.output("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		ref := strings.TrimSpace(string(out))
		// "refs/remotes/origin/main" → "main"
		if parts := strings.SplitN(ref, "/", 4); len(parts) == 4 {
			return parts[3], nil
		}
	}

	// Fall back to checking local branches.
	for _, name := range []string{"main", "master"} {
		if _, err := g.output("rev-parse", "--verify", "--end-of-options", name); err == nil {
			return name, nil
		}
	}
	return "", sdk.Errorf(sdk.ErrUnavailable, "no default branch was found (tried origin/HEAD, main and master).")
}

// Diff runs git diff between base and branch using three-dot notation.
func (g *Git) Diff(base, branch string, opts sdk.DiffOpts) (string, error) {
	if err := checkRefs(base, branch); err != nil {
		return "", err
	}
	args := []string{"diff"}
	if opts.Stat {
		args = append(args, "--stat")
	}
	args = append(args, "--end-of-options", base+"..."+branch)

	out, err := g.output(args...)
	if err != nil {
		return "", branchError(err, branch)
	}
	return strings.TrimRight(output(out), "\n"), nil
}

// Files returns the list of files changed between base and branch.
func (g *Git) Files(base, branch string) ([]string, error) {
	if err := checkRefs(base, branch); err != nil {
		return nil, err
	}
	out, err := g.output("diff", "--name-only", "--end-of-options", base+"..."+branch)
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
	if err := checkRefs(base, branch); err != nil {
		return nil, err
	}
	out, err := g.output("log", "--oneline", "--end-of-options", base+".."+branch)
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
	if err := checkRefs(branch); err != nil {
		return err
	}
	out, err := g.combined("checkout", "-b", branch)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "already exists") {
			return sdk.Errorf(sdk.ErrExists, "the branch %s already exists.", branch)
		}
		return fmt.Errorf("git checkout -b: %s", msg)
	}
	return nil
}

// RevCount returns the number of commits ahead and behind between
// branch and base. Ahead is commits on branch not on base; behind is
// commits on base not on branch.
func (g *Git) RevCount(base, branch string) (ahead, behind int, err error) {
	if err := checkRefs(base, branch); err != nil {
		return 0, 0, err
	}
	out, err := g.output("rev-list", "--count", "--left-right", "--end-of-options", base+"..."+branch)
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
	if err := checkPath(path); err != nil {
		return err
	}
	if err := checkRefs(branch); err != nil {
		return err
	}
	out, err := g.combined("worktree", "add", path, branch)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git worktree add: %s", msg)
		}
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

// WorktreeCreate creates a new branch and worktree in one operation.
// Uses "git worktree add -b <branch> <path>" so the main working
// directory is not affected.
func (g *Git) WorktreeCreate(path, branch string) error {
	if err := checkPath(path); err != nil {
		return err
	}
	if err := checkRefs(branch); err != nil {
		return err
	}
	out, err := g.combined("worktree", "add", "-b", branch, path)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git worktree create: %s", msg)
		}
		return fmt.Errorf("git worktree create: %w", err)
	}
	return nil
}

// WorktreeRemove removes a git worktree at the given path.
func (g *Git) WorktreeRemove(path string) error {
	if err := checkPath(path); err != nil {
		return err
	}
	out, err := g.combined("worktree", "remove", "--force", path)
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
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if strings.Contains(stderr, "unknown revision") {
			return sdk.Errorf(sdk.ErrNotFound, "git has no branch %s.", branch)
		}
		if stderr != "" {
			return fmt.Errorf("git: %s", stderr)
		}
	}
	return fmt.Errorf("running git: %w", err)
}
