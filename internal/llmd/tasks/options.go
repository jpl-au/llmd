// options.go defines option structs for task operations.

package tasks

import "github.com/jpl-au/llmd/pkg/model/core"

// AddOptions configures a task add operation.
type AddOptions struct {
	core.Origin
	Status     string
	Priority   int
	AssignedTo string
	Branch     string
	Path       string // Existing store document to use as spec
}

// ListOptions filters the task listing. All zero-valued fields mean
// "no filter" — the query returns all non-deleted tasks. Filters are
// combined with AND.
type ListOptions struct {
	Status     string
	AssignedTo string
	Priority   int // 0 = all
	Branch     string
	SinceMS    int64 // Unix millis; 0 = no filter
}

// SetOptions configures which fields to update.
type SetOptions struct {
	Title      *string
	Priority   *int
	Position   *int
	AssignedTo *string
	Branch     *string
	Flag       string // Flag to add
	Unflag     string // Flag to remove
}
