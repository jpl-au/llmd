// task_git.go provides git-aware task subcommands: start, finish, branch,
// diff, files, commits.
//
// These commands bridge the task board with git, allowing users to
// associate a task with a branch and then view what changed. All git
// operations degrade gracefully - if git is unavailable, commands either
// skip the git parts (finish) or return a clear error.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var taskStartFlags = []sdk.Flag{
	{Name: "column", Type: "string"},
	{Name: "assign", Type: "string", Desc: "Spawn an agent for this task"},
	{Name: "budget", Type: "string", Desc: "Max agent budget in USD for this spawn"},
}

// taskStart moves a task to in-progress and records the current git
// branch if available. When --assign is set, spawns an agent process
// in a git worktree to work on the task.
func taskStart(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskStartFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task start: %w", err)
	}
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}
	if key == "" {
		return nil, fmt.Errorf("task start: %w: id", sdk.ErrMissingArg)
	}

	agent := flags.String("assign")
	if agent != "" {
		var opts sdk.SpawnOpts
		if b := flags.String("budget"); b != "" {
			if _, err := fmt.Sscanf(b, "%f", &opts.MaxBudget); err != nil {
				return nil, fmt.Errorf("task start: invalid budget %q: %w", b, err)
			}
		}
		// Spawn an agent - this handles branch creation, worktree,
		// context assembly, process start, and moving to in-progress.
		r, err := ctx.Agents.Spawn(key, agent, ctx.Author, opts)
		if err != nil {
			return nil, fmt.Errorf("task start: %w", err)
		}
		text := fmt.Sprintf("Started %s with agent %s (run %s, pid %d)\n  Worktree: %s\n  Branch:   %s",
			key, agent, r.Key, r.PID, r.Worktree, r.Branch)
		return sdk.Result{Text: text, Data: r}, nil
	}

	// No agent - standard start.
	opts := sdk.StartOpts{Column: flags.String("column")}
	t, err := ctx.Tasks.Start(key, ctx.Author, opts)
	if err != nil {
		return nil, fmt.Errorf("task start: %w", err)
	}

	if t.Branch != "" {
		return sdk.Text(fmt.Sprintf("Started %s on branch %s", t.Key, t.Branch)), nil
	}
	return sdk.Text(fmt.Sprintf("Started %s", t.Key)), nil
}

var taskDiffFlags = []sdk.Flag{
	{Name: "base", Type: "string"},
	{Name: "stat", Type: "bool"},
}

// taskDiff shows the git diff for a task's branch against the default branch.
// If no task ID is given, auto-detects from the current branch.
func taskDiff(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskDiffFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task diff: %w", err)
	}
	base := flags.String("base")
	stat := flags.Bool("stat")
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}

	t, err := resolveTask(ctx, "task diff", key)
	if err != nil {
		return nil, err
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task diff: task has no branch - use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = ctx.Git.DefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task diff: %w - use --base to specify", err)
		}
	}

	output, err := ctx.Git.Diff(base, t.Branch, sdk.DiffOpts{Stat: stat})
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

var taskFilesFlags = []sdk.Flag{
	{Name: "base", Type: "string"},
}

// taskFiles lists files changed on a task's branch.
// If no task ID is given, auto-detects from the current branch.
func taskFiles(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskFilesFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task files: %w", err)
	}
	base := flags.String("base")
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}

	t, err := resolveTask(ctx, "task files", key)
	if err != nil {
		return nil, err
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task files: task has no branch - use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = ctx.Git.DefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task files: %w - use --base to specify", err)
		}
	}

	files, err := ctx.Git.Files(base, t.Branch)
	if err != nil {
		return nil, fmt.Errorf("task files: %w", err)
	}

	if len(files) == 0 {
		return sdk.Text("No changed files"), nil
	}

	return sdk.Result{Text: strings.Join(files, "\n"), Data: files}, nil
}

var taskFinishFlags = []sdk.Flag{
	{Name: "column", Type: "string"},
	{Name: "base", Type: "string"},
}

// taskFinish moves a task to done and shows a summary. Delegates to
// ctx.Tasks.Finish which handles git summary collection.
func taskFinish(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskFinishFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task finish: %w", err)
	}
	opts := sdk.FinishOpts{
		Column: flags.String("column"),
		Base:   flags.String("base"),
	}
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}

	t, err := resolveTask(ctx, "task finish", key)
	if err != nil {
		return nil, err
	}

	result, err := ctx.Tasks.Finish(t.Key, ctx.Author, opts)
	if err != nil {
		return nil, fmt.Errorf("task finish: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Finished %s \"%s\"", result.Task.Key, result.Task.Title)
	if result.FilesChanged > 0 {
		fmt.Fprintf(&b, "\n  %d file(s) changed", result.FilesChanged)
	}
	if result.Commits > 0 {
		fmt.Fprintf(&b, "\n  %d commit(s)", result.Commits)
	}

	return sdk.Text(b.String()), nil
}

var taskBranchFlags = []sdk.Flag{
	{Name: "name", Type: "string"},
	{Name: "column", Type: "string"},
}

// taskBranch creates a git branch from a task's title, checks it out,
// records the branch on the task, and moves it to in-progress.
// Delegates to ctx.Tasks.StartBranch.
func taskBranch(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskBranchFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}
	opts := sdk.StartBranchOpts{
		Name:   flags.String("name"),
		Column: flags.String("column"),
	}
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}
	if key == "" {
		return nil, fmt.Errorf("task branch: %w: id", sdk.ErrMissingArg)
	}

	t, err := ctx.Tasks.StartBranch(key, ctx.Author, opts)
	if err != nil {
		return nil, fmt.Errorf("task branch: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Created branch %s for %s \"%s\"", t.Branch, t.Key, t.Title)), nil
}

var taskCommitsFlags = []sdk.Flag{
	{Name: "base", Type: "string"},
}

// taskCommits lists commits on a task's branch that aren't on the base branch.
func taskCommits(ctx sdk.Context, args []string) (sdk.Response, error) {
	if err := ctx.Git.Available(); err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}

	flags, positional, err := sdk.ParseArgs(taskCommitsFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}
	base := flags.String("base")
	var key string
	if len(positional) > 0 {
		key = positional[0]
	}

	t, err := resolveTask(ctx, "task commits", key)
	if err != nil {
		return nil, err
	}
	if t.Branch == "" {
		return nil, fmt.Errorf("task commits: task has no branch - use 'task start' or 'task set --branch'")
	}

	if base == "" {
		base, err = ctx.Git.DefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("task commits: %w - use --base to specify", err)
		}
	}

	commits, err := ctx.Git.Commits(base, t.Branch)
	if err != nil {
		return nil, fmt.Errorf("task commits: %w", err)
	}

	if len(commits) == 0 {
		return sdk.Text("No commits"), nil
	}

	return sdk.Result{Text: strings.Join(commits, "\n"), Data: commits}, nil
}

// resolveTask looks up a task by key, or auto-detects from the current
// branch when key is empty. The cmd parameter is used for error messages.
func resolveTask(ctx sdk.Context, cmd, key string) (*sdk.Task, error) {
	if key != "" {
		t, err := ctx.Tasks.Read(key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
		return t, nil
	}
	branch, err := ctx.Git.Branch()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	t, err := ctx.Tasks.ByBranch(branch)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	return t, nil
}
