// Package task provides the Task model for kanban task management.
package task

import "github.com/jpl-au/llmd/pkg/model/core"

// Task represents a kanban task stored in the tasks table.
type Task struct {
	ID         int64
	Key        string
	Title      string
	Status     string
	Priority   int
	Position   int
	AssignedTo string
	Flags      string // Comma-separated: "blocked", "hold", "blocked,hold"
	Path       string // Document path in the content table
	core.Origin
	CreatedAt int64
	DeletedAt *int64
}
