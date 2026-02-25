// Package task provides the Task model for task management.
//
// Tasks are mutable board items stored in a dedicated tasks table. Unlike
// documents (which are append-only with version history), task fields like
// status, priority, position, and assignment are updated in place. Every
// mutation is recorded in the audit log for observability.
//
// Each task has a corresponding document in the content table (its "spec")
// linked by Path. Tasks in backlog may have an empty spec; moving out of
// backlog requires the spec to have content beyond the template heading.
package task

import "github.com/jpl-au/llmd/pkg/model/core"

// Task represents a task stored in the tasks table.
//
// The struct maps directly to a database row. Nullable columns use Go
// zero values (empty string for NULLs) after scanning, except DeletedAt
// which uses a pointer to distinguish "not deleted" from "deleted at
// epoch zero".
type Task struct {
	// ID is the auto-increment database row ID.
	// Not typically used in application code; use Key instead.
	ID int64

	// Key is the stable 9-character base36 identifier.
	// Unique across all tasks and never changes.
	Key string

	// Title is the human-readable summary.
	Title string

	// Status is the board column (e.g. "backlog", "in-progress", "done").
	Status string

	// Priority is the numeric priority level. Zero means unset.
	Priority int

	// Position is the sort order within the column. Lower values first.
	Position int

	// AssignedTo is the person responsible. Empty means unassigned.
	// Stored as sql.NullString in the database.
	AssignedTo string

	// Branch is the git branch associated with this task. Empty means
	// no branch. Set by "task start" or "task set --branch".
	Branch string

	// Flags is a comma-separated set of flags (e.g. "blocked", "hold",
	// "blocked,hold"). Stored as sql.NullString; empty string means
	// no flags.
	Flags string

	// Path is the document path in the content table that holds this
	// task's spec body. Convention: "tasks/<slug>" for auto-created
	// specs, or an arbitrary path for linked documents.
	Path string

	// Origin embeds authorship and provenance fields (Author, Source,
	// Message). Author is who created the task; Source is "cli", "mcp",
	// etc.
	core.Origin

	// CreatedAt is the Unix timestamp (milliseconds) when this task
	// was created.
	CreatedAt int64

	// DeletedAt is the Unix timestamp (milliseconds) when soft-deleted,
	// or nil if the task is active. Soft-deleted tasks are excluded from
	// normal queries but can be restored.
	DeletedAt *int64
}
