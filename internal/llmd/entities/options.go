package entities

import "github.com/jpl-au/llmd/internal/llmd/core"

// WriteOptions configures a write operation.
type WriteOptions struct {
	core.WriteContext
	Relation string // Optional relation (key, path, or identifier)
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	core.WriteContext
}

// ListOptions configures a list operation.
type ListOptions struct {
	Relation string // Filter by relation (empty = all)
	Limit    int
}
