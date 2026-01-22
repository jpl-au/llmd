package entities

import "github.com/jpl-au/llmd/pkg/model/core"

// WriteOptions configures a write operation.
type WriteOptions struct {
	core.Origin
	Relation string // Optional relation (key, path, or identifier)
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	core.Origin
}

// ListOptions configures a list operation.
type ListOptions struct {
	Relation string // Filter by relation (empty = all)
	Limit    int
}
