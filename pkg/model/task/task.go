// Package task provides the Task model for task management.
package task

import "github.com/jpl-au/llmd/pkg/model/core"

// Task represents a task stored in the tasks table.
type Task struct {
	ID         int64
	Key        string
	Title      string
	Status     string
	Priority   int
	Position   int
	AssignedTo string
	Branch     string // Git branch associated with this task
	Flags      string // Comma-separated: "blocked", "hold", "blocked,hold"
	Path       string // Document path in the content table
	core.Origin
	CreatedAt int64
	DeletedAt *int64
}
