package sdk

import "time"

// TaskStore is the task management interface for the board.
//
// Tasks live in columns (like a kanban board). Each task has a title,
// optional spec document, priority, flags, and assignee. Tasks move
// between columns via [TaskStore.Move], and the column set itself is
// customisable via AddColumn, RemoveColumn, and MoveColumn.
//
// A task's spec is a document in the store linked by path. Tasks in
// the backlog column may have no spec; moving out of backlog requires
// one (enforced by Move). The spec can be created inline during Add
// or linked to an existing document via [TaskAddOpts.Path].
type TaskStore interface {
	// Add creates a new task with the given title and optional body.
	Add(title string, body []byte, opts TaskAddOpts) (*Task, error)

	// Read returns a task by its key.
	Read(key string) (*Task, error)

	// List returns tasks, optionally filtered by column, assignee,
	// or priority.
	List(opts TaskListOpts) ([]*Task, error)

	// Move changes a task's column.
	Move(key, column, author string) error

	// Set updates task metadata (title, priority, flags, etc.).
	Set(key, author string, opts TaskSetOpts) error

	// Delete soft-deletes a task.
	Delete(key, author string) (*Task, error)

	// Restore undeletes a soft-deleted task.
	Restore(key, author string) (*Task, error)

	// Columns returns the board columns in order.
	Columns() ([]string, error)

	// AddColumn adds a new column. When after is non-empty, the
	// column is inserted after the named column.
	AddColumn(name, after, author string) error

	// RemoveColumn removes an empty column.
	RemoveColumn(name, author string) error

	// MoveColumn reorders a column to appear after the named column.
	MoveColumn(name, after, author string) error

	// Start moves a task to a column and optionally records the current
	// git branch. Returns the updated task.
	Start(key, author string, opts StartOpts) (*Task, error)

	// StartBranch creates a git branch from the task title (or custom
	// name), records it on the task, and moves to a column. Returns
	// the updated task.
	StartBranch(key, author string, opts StartBranchOpts) (*Task, error)

	// Finish moves a task to done and returns a summary. If the task
	// has a branch and git is available, the summary includes file
	// and commit counts.
	Finish(key, author string, opts FinishOpts) (*FinishResult, error)

	// ByBranch returns the task linked to the given branch name.
	ByBranch(branch string) (*Task, error)

	// CheckSpecs reports which tasks have a document at their spec
	// path. Returns a map from task key to true for tasks with specs.
	CheckSpecs(tasks []*Task) (map[string]bool, error)

	// Link creates a directed link from a task's spec document to
	// another document in the store.
	Link(key, target, author string) error

	// Links returns links for a task's spec document. Dir controls
	// direction: "out" (default), "in", or "both".
	Links(key, dir string) ([]Link, error)

	// Log returns audit events for a task, newest first.
	// Limit 0 means all events.
	Log(key string, limit int) ([]TaskEvent, error)
}

// Task represents a task on the board.
//
// Tasks are the SDK's view of a board item. Unlike documents, tasks are
// mutable - status, priority, position, assignee, branch, and flags can
// all be updated in place. Every mutation is recorded in the audit log
// (see [TaskStore.Log]).
type Task struct {
	// Key is the stable 9-character base36 identifier for this task.
	// Unique across all tasks and never changes.
	Key string

	// Title is the human-readable summary of the task.
	// Displayed in board views and tables.
	Title string

	// Status is the column name this task belongs to (e.g. "backlog",
	// "in-progress", "done"). Controlled by [TaskStore.Move].
	Status string

	// Priority is a numeric priority level. Higher values indicate
	// higher importance. Zero means no priority set.
	Priority int

	// Position is the sort order within the task's column. Lower
	// values appear first. Managed automatically by Add and Move;
	// can be overridden via [TaskSetOpts.Position].
	Position int

	// AssignedTo is the person responsible for this task.
	// Empty string means unassigned.
	AssignedTo string

	// Branch is the git branch associated with this task.
	// Set by "task start" or via [TaskSetOpts.Branch]. Used by
	// "task diff" and "task files" to show git changes.
	Branch string

	// Flags is a comma-separated set of flags (e.g. "blocked",
	// "hold", "blocked,hold"). Flags are free-form strings managed
	// via [TaskSetOpts.Flag] and [TaskSetOpts.Unflag].
	Flags string

	// Path is the store document path for this task's spec body.
	// Points to a document in the content table. May reference a
	// document that has not yet been written (backlog tasks).
	Path string

	// Author is the user who created this task.
	Author string

	// CreatedAt is the Unix timestamp (milliseconds) when this task
	// was created.
	CreatedAt int64
}

// TaskAddOpts configures a task add operation.
//
// All fields are optional. When Status is empty, the task is placed in
// "backlog". When Path is set, the task links to an existing store
// document instead of creating a new one. Author is required for all
// write operations.
type TaskAddOpts struct {
	// Status is the initial column. Defaults to "backlog".
	Status string

	// Priority sets the task priority. Zero means no priority.
	Priority int

	// AssignedTo sets the initial assignee.
	AssignedTo string

	// Branch associates a git branch with the task at creation time.
	Branch string

	// Path links the task to an existing store document as its spec.
	// Mutually exclusive with providing a body to Add.
	Path string

	// Author identifies who is creating this task. Required.
	Author string
}

// TaskListOpts filters the task list. All fields are optional; when all
// are zero-valued, all non-deleted tasks are returned. Filters are
// combined with AND - setting both Status and AssignedTo returns only
// tasks matching both criteria.
type TaskListOpts struct {
	// Status filters to tasks in this column (e.g. "in-progress").
	Status string

	// AssignedTo filters to tasks assigned to this person.
	AssignedTo string

	// Priority filters to tasks with this exact priority.
	// Zero means no filter (returns all priorities).
	Priority int

	// Branch filters to tasks linked to this git branch.
	Branch string

	// Since filters to tasks created after this time. Zero means
	// no filter.
	Since time.Time
}

// TaskSetOpts configures which task fields to update. Pointer fields
// use nil to mean "don't change" and a non-nil value to set. This
// allows clearing a field by passing a pointer to the zero value
// (e.g. &"" to unassign). Each non-nil field generates its own audit
// log entry showing the old and new values.
type TaskSetOpts struct {
	// Title replaces the task title.
	Title *string

	// Priority sets the numeric priority.
	Priority *int

	// Position moves the task within its column. Other tasks in the
	// column are renumbered to accommodate the new position.
	Position *int

	// AssignedTo changes the assignee. Pointer to empty string unassigns.
	AssignedTo *string

	// Branch changes the associated git branch.
	// Pointer to empty string removes the branch association.
	Branch *string

	// Flag adds a flag to the task's flag set (e.g. "blocked", "hold").
	// No-op if the flag is already present.
	Flag string

	// Unflag removes a flag from the task's flag set.
	// No-op if the flag is not present.
	Unflag string
}

// StartOpts configures a task start operation.
type StartOpts struct {
	// Column is the target column. Defaults to "in-progress".
	Column string
}

// StartBranchOpts configures a task start-with-branch operation.
type StartBranchOpts struct {
	// Name is the git branch name. When empty, a name is generated
	// from the task title as "task/<slug>".
	Name string

	// Column is the target column. Defaults to "in-progress".
	Column string
}

// FinishOpts configures a task finish operation.
type FinishOpts struct {
	// Column is the target column. Defaults to "done".
	Column string

	// Base is the git base branch for the summary diff. When empty,
	// the default branch is detected automatically.
	Base string
}

// FinishResult contains the outcome of finishing a task.
type FinishResult struct {
	// Task is the updated task after moving to the done column.
	Task *Task

	// FilesChanged is the number of files changed on the task's
	// branch relative to the base. Zero when git is unavailable
	// or the task has no branch.
	FilesChanged int

	// Commits is the number of commits on the task's branch that
	// are not on the base. Zero when git is unavailable or the
	// task has no branch.
	Commits int
}

// TaskEvent is a single audit log entry for a task. Every mutation to a
// task - creation, movement, field edits, flagging, deletion, and
// restoration - writes a TaskEvent. Events are returned newest-first
// by [TaskStore.Log].
type TaskEvent struct {
	// Timestamp is the Unix timestamp (milliseconds) when the event occurred.
	Timestamp int64

	// Subject is the task key this event applies to.
	Subject string

	// Actor is the user who performed the action.
	Actor string

	// Action describes what changed. Standard actions: "created",
	// "moved", "edited:title", "edited:priority", "edited:assigned_to",
	// "edited:branch", "edited:position", "flagged", "unflagged",
	// "deleted", "restored".
	Action string

	// OldValue is the previous value before the change. Empty for
	// creation events.
	OldValue string

	// NewValue is the value after the change. Empty for deletion events.
	NewValue string
}
