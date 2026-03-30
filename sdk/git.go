package sdk

// GitStore provides git repository operations. All consumers (CLI,
// MCP, HTTP) access git through this interface.
type GitStore interface {
	// Available checks that git is installed and the working directory
	// is inside a git repository. Returns nil if both conditions are
	// met, or a descriptive error explaining what's missing.
	Available() error

	// Branch returns the current git branch name.
	Branch() (string, error)

	// DefaultBranch returns the name of the default branch. It tries
	// the remote HEAD symbolic ref first (works when origin is
	// configured), then falls back to checking whether main or master
	// exist locally.
	DefaultBranch() (string, error)

	// Diff runs git diff between base and branch using three-dot
	// notation.
	Diff(base, branch string, opts DiffOpts) (string, error)

	// Files returns the list of files changed between base and branch.
	Files(base, branch string) ([]string, error)

	// Commits returns commit summaries on branch that are not on base.
	// Each entry is a single-line "hash subject" string.
	Commits(base, branch string) ([]string, error)

	// CheckoutNew creates a new branch and switches to it.
	CheckoutNew(branch string) error

	// RevCount returns the number of commits ahead and behind between
	// branch and base.
	RevCount(base, branch string) (ahead, behind int, err error)

	// WorktreeAdd creates a new git worktree at the given path,
	// checked out to the specified branch. The branch must already
	// exist.
	WorktreeAdd(path, branch string) error

	// WorktreeCreate creates a new branch and worktree in one
	// operation. Unlike CheckoutNew + WorktreeAdd, this does not
	// affect the main working directory.
	WorktreeCreate(path, branch string) error

	// WorktreeRemove removes a git worktree at the given path. The
	// worktree directory is deleted from disk.
	WorktreeRemove(path string) error
}

// DiffOpts configures a git diff operation.
type DiffOpts struct {
	Stat bool // Show diffstat instead of full diff
}
