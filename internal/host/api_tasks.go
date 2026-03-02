package host

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/internal/validate"
	"github.com/jpl-au/llmd/pkg/model/task"
	"github.com/jpl-au/llmd/sdk"
)

// taskAPI implements [sdk.TaskStore] by delegating to the internal tasks
// package. It translates between SDK types (flat structs with string
// fields) and internal types (sql.NullString, core.Origin, etc.).
type taskAPI struct {
	store *llmd.Store
	lim   validate.Limits
}

// newTaskAPI creates a task API bridge wrapping the given store.
// The returned value satisfies [sdk.TaskStore] and is assigned to the
// sdk.Tasks global by [New].
func newTaskAPI(store *llmd.Store, lim validate.Limits) *taskAPI {
	return &taskAPI{store: store, lim: lim}
}

// taskErr translates internal task errors to SDK sentinel errors.
// ErrNotFound, ErrMissingTitle, ErrInvalidCol, and ErrNoSpec are mapped;
// all other errors pass through unchanged.
func taskErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		return fmt.Errorf("%w: %v", sdk.ErrNotFound, err)
	case errors.Is(err, tasks.ErrNoSpec):
		return fmt.Errorf("%w: %v", sdk.ErrNoSpec, err)
	case errors.Is(err, tasks.ErrMissingTitle):
		return fmt.Errorf("%w: %v", sdk.ErrMissingArg, err)
	case errors.Is(err, tasks.ErrInvalidCol):
		return fmt.Errorf("%w: %v", sdk.ErrInvalidArg, err)
	default:
		return err
	}
}

// taskToSDK converts an internal task model to the SDK representation.
// Nullable fields (AssignedTo, Branch, Flags) have already been scanned
// to Go zero values by the tasks package, so the mapping is direct.
func taskToSDK(t *task.Task) *sdk.Task {
	return &sdk.Task{
		Key:        t.Key,
		Title:      t.Title,
		Status:     t.Status,
		Priority:   t.Priority,
		Position:   t.Position,
		AssignedTo: t.AssignedTo,
		Branch:     t.Branch,
		Flags:      t.Flags,
		Path:       t.Path,
		Author:     t.Author,
		CreatedAt:  t.CreatedAt,
	}
}

// Add creates a new task with the given title and optional spec body.
// Maps SDK options to internal AddOptions and stamps a CLI origin.
func (a *taskAPI) Add(title string, body []byte, opts sdk.TaskAddOpts) (*sdk.Task, error) {
	if err := validate.Text(title, "title"); err != nil {
		return nil, err
	}
	if err := validate.Content(body, a.lim); err != nil {
		return nil, err
	}
	t, err := a.store.Tasks.Add(context.Background(), title, body, tasks.AddOptions{
		Origin:     origin(opts.Author),
		Status:     opts.Status,
		Priority:   opts.Priority,
		AssignedTo: opts.AssignedTo,
		Branch:     opts.Branch,
		Path:       opts.Path,
	})
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// Read returns a single task by its key. Returns sdk.ErrNotFound if the
// task does not exist or has been deleted.
func (a *taskAPI) Read(key string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// List returns all non-deleted tasks matching the filter criteria.
// Results are ordered by position then creation time within each column.
func (a *taskAPI) List(opts sdk.TaskListOpts) ([]*sdk.Task, error) {
	tt, err := a.store.Tasks.List(context.Background(), tasks.ListOptions{
		Status:     opts.Status,
		AssignedTo: opts.AssignedTo,
		Priority:   opts.Priority,
		Branch:     opts.Branch,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*sdk.Task, len(tt))
	for i, t := range tt {
		out[i] = taskToSDK(t)
	}
	return out, nil
}

// Move changes a task's column. Translates internal task errors to SDK
// sentinels (ErrNotFound, ErrNoSpec, ErrInvalidArg).
func (a *taskAPI) Move(key, column, author string) error {
	return taskErr(a.store.Tasks.Move(context.Background(), key, column, author))
}

// Set updates task metadata. Maps SDK pointer fields directly to
// internal SetOptions — the nil-means-no-change convention is shared.
func (a *taskAPI) Set(key, author string, opts sdk.TaskSetOpts) error {
	return taskErr(a.store.Tasks.Set(context.Background(), key, author, tasks.SetOptions{
		Title:      opts.Title,
		Priority:   opts.Priority,
		Position:   opts.Position,
		AssignedTo: opts.AssignedTo,
		Branch:     opts.Branch,
		Flag:       opts.Flag,
		Unflag:     opts.Unflag,
	}))
}

// Delete soft-deletes a task. Returns the task as it was before
// deletion so the caller can display confirmation with the title.
func (a *taskAPI) Delete(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Delete(context.Background(), key, author)
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// Restore undeletes a soft-deleted task. Returns the restored task
// so the caller can confirm which task was recovered.
func (a *taskAPI) Restore(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Restore(context.Background(), key, author)
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// Columns returns the board column names in display order.
func (a *taskAPI) Columns() ([]string, error) {
	return a.store.Tasks.Columns(context.Background())
}

// AddColumn adds a new column to the board. When after is non-empty,
// the column is inserted after the named column; otherwise it is appended.
func (a *taskAPI) AddColumn(name, after, author string) error {
	if err := validate.Text(name, "column name"); err != nil {
		return err
	}
	return a.store.Tasks.AddColumn(context.Background(), name, after, author)
}

// RemoveColumn removes a column from the board. Fails if the column
// still contains tasks — they must be moved or deleted first.
func (a *taskAPI) RemoveColumn(name, author string) error {
	return a.store.Tasks.RemoveColumn(context.Background(), name, author)
}

// MoveColumn reorders a column to appear after the named column.
func (a *taskAPI) MoveColumn(name, after, author string) error {
	return a.store.Tasks.MoveColumn(context.Background(), name, after, author)
}

// Start moves a task to a column and records the current git branch
// when available. Git is optional — the task starts regardless.
func (a *taskAPI) Start(key, author string, opts sdk.StartOpts) (*sdk.Task, error) {
	col := opts.Column
	if col == "" {
		col = "in-progress"
	}
	if err := taskErr(a.store.Tasks.Move(context.Background(), key, col, author)); err != nil {
		return nil, err
	}

	// Best-effort: record current branch if git is available.
	if branch, err := sdk.Git.Branch(); err == nil {
		_ = taskErr(a.store.Tasks.Set(context.Background(), key, author, tasks.SetOptions{
			Branch: &branch,
		}))
	}

	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// StartBranch creates a git branch from the task title (or custom name),
// records it on the task, and moves to a column.
func (a *taskAPI) StartBranch(key, author string, opts sdk.StartBranchOpts) (*sdk.Task, error) {
	if err := sdk.Git.Available(); err != nil {
		return nil, err
	}

	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}
	if t.Branch != "" {
		return nil, fmt.Errorf("%w: task already has branch %q", sdk.ErrInvalidArg, t.Branch)
	}

	name := opts.Name
	if name == "" {
		name = "task/" + branchSlug(t.Title)
	}

	if err := sdk.Git.CheckoutNew(name); err != nil {
		return nil, err
	}

	if err := taskErr(a.store.Tasks.Set(context.Background(), key, author, tasks.SetOptions{
		Branch: &name,
	})); err != nil {
		return nil, err
	}

	col := opts.Column
	if col == "" {
		col = "in-progress"
	}
	if err := taskErr(a.store.Tasks.Move(context.Background(), key, col, author)); err != nil {
		return nil, err
	}

	t, err = a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}
	return taskToSDK(t), nil
}

// Finish moves a task to done and returns a summary with optional git
// statistics. Git is optional — the task moves regardless.
func (a *taskAPI) Finish(key, author string, opts sdk.FinishOpts) (*sdk.FinishResult, error) {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}

	col := opts.Column
	if col == "" {
		col = "done"
	}
	if err := taskErr(a.store.Tasks.Move(context.Background(), key, col, author)); err != nil {
		return nil, err
	}

	t, err = a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}

	result := &sdk.FinishResult{Task: taskToSDK(t)}

	// Git summary — best effort, skip if unavailable.
	if t.Branch != "" && sdk.Git.Available() == nil {
		base := opts.Base
		if base == "" {
			base, _ = sdk.Git.DefaultBranch()
		}
		if base != "" {
			if files, err := sdk.Git.Files(base, t.Branch); err == nil {
				result.FilesChanged = len(files)
			}
			if commits, err := sdk.Git.Commits(base, t.Branch); err == nil {
				result.Commits = len(commits)
			}
		}
	}

	return result, nil
}

// ByBranch returns the task linked to the given branch name.
func (a *taskAPI) ByBranch(branch string) (*sdk.Task, error) {
	tt, err := a.store.Tasks.List(context.Background(), tasks.ListOptions{
		Branch: branch,
	})
	if err != nil {
		return nil, err
	}
	if len(tt) == 0 {
		return nil, fmt.Errorf("%w: no task linked to branch %q", sdk.ErrNotFound, branch)
	}
	return taskToSDK(tt[0]), nil
}

// CheckSpecs reports which tasks have a document at their spec path.
// Each unique path is checked at most once.
func (a *taskAPI) CheckSpecs(tasks []*sdk.Task) (map[string]bool, error) {
	paths := make(map[string]bool)
	for _, t := range tasks {
		if t.Path == "" {
			continue
		}
		if _, checked := paths[t.Path]; !checked {
			ok, err := sdk.Documents.Exists(t.Path)
			paths[t.Path] = err == nil && ok
		}
	}
	m := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if paths[t.Path] {
			m[t.Key] = true
		}
	}
	return m, nil
}

// Link creates a directed link from a task's spec document to another
// document.
func (a *taskAPI) Link(key, target, author string) error {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return taskErr(err)
	}
	if t.Path == "" {
		return fmt.Errorf("%w: task has no spec document", sdk.ErrNoSpec)
	}
	return sdk.Links.Add(t.Path, target, "", author)
}

// Links returns links for a task's spec document.
func (a *taskAPI) Links(key, dir string) ([]sdk.Link, error) {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, taskErr(err)
	}
	if t.Path == "" {
		return nil, nil
	}
	links, err := sdk.Links.List(t.Path, dir)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// branchSlug converts a title to a git-friendly branch component.
// Letters and digits are kept; everything else becomes a dash. Runs of
// dashes collapse and trailing dashes are trimmed.
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

// Log returns audit events for a task, newest first. Converts internal
// audit.Event structs to SDK TaskEvent structs.
func (a *taskAPI) Log(key string, limit int) ([]sdk.TaskEvent, error) {
	events, err := a.store.Tasks.Log(context.Background(), key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.TaskEvent, len(events))
	for i, e := range events {
		out[i] = sdk.TaskEvent{
			Timestamp: e.Timestamp,
			Subject:   e.Subject,
			Actor:     e.Actor,
			Action:    e.Action,
			OldValue:  e.OldValue,
			NewValue:  e.NewValue,
		}
	}
	return out, nil
}
