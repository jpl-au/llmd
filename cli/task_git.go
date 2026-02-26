// task_git.go provides git-aware task subcommands: start, finish, branch,
// diff, files, commits.
//
// These commands bridge the task board with git, allowing users to
// associate a task with a branch and then view what changed. All git
// operations degrade gracefully — if git is unavailable, commands either
// skip the git parts (finish) or return a clear error.

package cli

import (
	"fmt"
	"strings"
	"unicode"

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

// taskFinish moves a task to done and shows a summary. If the task has
// a branch and git is available, the summary includes file and commit
// counts. Without git, the task is still moved — git is optional.
func taskFinish(ctx sdk.Context, args []string) (sdk.Response, error) {
	column := "done"
	var key, base string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task finish: --column requires a value")
			}
			column = args[i]
		case "--base":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task finish: --base requires a value")
			}
			base = args[i]
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task finish: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(key)
	if err != nil {
		return nil, fmt.Errorf("task finish: %w", err)
	}

	if err := sdk.Tasks.Move(key, column, ctx.Author); err != nil {
		return nil, fmt.Errorf("task finish: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Finished %s \"%s\"", t.Key, t.Title)

	// Git summary — best effort, skip if unavailable.
	if t.Branch != "" && gitAvailable() == nil {
		if base == "" {
			base, _ = gitDefaultBranch()
		}
		if base != "" {
			if files, err := gitFiles(base, t.Branch); err == nil && len(files) > 0 {
				fmt.Fprintf(&b, "\n  %d file(s) changed", len(files))
			}
			if commits, err := gitCommits(base, t.Branch); err == nil && len(commits) > 0 {
				fmt.Fprintf(&b, "\n  %d commit(s)", len(commits))
			}
		}
	}

	return sdk.Text(b.String()), nil
}

// taskBranch creates a git branch from a task's title, checks it out,
// records the branch on the task, and moves it to in-progress.
func taskBranch(ctx sdk.Context, args []string) (sdk.Response, error) {
	if err := gitAvailable(); err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}

	column := "in-progress"
	var key, name string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task branch: --name requires a value")
			}
			name = args[i]
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task branch: --column requires a value")
			}
			column = args[i]
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task branch: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(key)
	if err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}
	if t.Branch != "" {
		return nil, fmt.Errorf("task branch: task already has branch %q — use 'task set --branch' to change", t.Branch)
	}

	if name == "" {
		name = "task/" + branchSlug(t.Title)
	}

	if err := gitCheckoutNew(name); err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}

	if err := sdk.Tasks.Set(key, ctx.Author, sdk.TaskSetOpts{Branch: &name}); err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}

	if err := sdk.Tasks.Move(key, column, ctx.Author); err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Created branch %s for %s \"%s\"", name, t.Key, t.Title)), nil
}

// taskCommits lists commits on a task's branch that aren't on the base branch.
func taskCommits(_ sdk.Context, args []string) (sdk.Response, error) {
	if err := gitAvailable(); err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}

	var key, base string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task commits: --base requires a value")
			}
			base = args[i]
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	if key == "" {
		return nil, fmt.Errorf("task commits: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(key)
	if err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task commits: task has no branch — use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = gitDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task commits: %w — use --base to specify", err)
		}
	}

	commits, err := gitCommits(base, t.Branch)
	if err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}

	if len(commits) == 0 {
		return sdk.Text("No commits"), nil
	}

	return sdk.Result{Text: strings.Join(commits, "\n"), Data: commits}, nil
}

// branchSlug converts a title to a git-friendly branch component.
func branchSlug(title string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prev = r
		case prev != '-':
			b.WriteByte('-')
			prev = '-'
		}
	}
	return strings.TrimRight(b.String(), "-")
}
