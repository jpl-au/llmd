package sdk

// TaskStore is the task management interface. It covers creating,
// reading, updating, and organising tasks on the board.
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

	// Log returns audit events for a task, newest first.
	// Limit 0 means all events.
	Log(key string, limit int) ([]TaskEvent, error)
}

// Task represents a task on the board.
type Task struct {
	Key        string
	Title      string
	Status     string
	Priority   int
	Position   int
	AssignedTo string
	Branch     string
	Flags      string
	Path       string
	Author     string
	CreatedAt  int64
}

// TaskAddOpts configures a task add operation.
type TaskAddOpts struct {
	Status     string
	Priority   int
	AssignedTo string
	Branch     string
	Path       string // Existing store document to use as spec
	Author     string
}

// TaskListOpts configures a task list operation.
type TaskListOpts struct {
	Status     string
	AssignedTo string
	Priority   int
}

// TaskSetOpts configures which task fields to update.
type TaskSetOpts struct {
	Title      *string
	Priority   *int
	Position   *int
	AssignedTo *string
	Branch     *string
	Flag       string
	Unflag     string
}

// TaskEvent is a single audit log entry for a task.
type TaskEvent struct {
	Timestamp int64
	Subject   string // task key
	Actor     string
	Action    string
	OldValue  string
	NewValue  string
}
