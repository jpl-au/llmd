package documents

import "github.com/jpl-au/llmd/internal/llmd/core"

// WriteOptions configures a write operation.
type WriteOptions struct {
	core.WriteContext
}

// ReadOptions configures a read operation.
type ReadOptions struct {
	Version *int // nil = latest, otherwise specific version
}

// ListOptions configures a list operation.
type ListOptions struct {
	Prefix         string // Filter by path prefix
	IncludeDeleted bool   // Include soft-deleted documents
	Limit          int    // Max results (0 = no limit)
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	core.WriteContext
}

// RestoreOptions configures a restore operation.
type RestoreOptions struct {
	core.WriteContext
}

// EditOptions configures an edit operation.
type EditOptions struct {
	core.WriteContext
	ReplaceAll bool // Replace all occurrences, not just first
}

// MoveOptions configures a move operation.
type MoveOptions struct {
	core.WriteContext
}

// CopyOptions configures a copy operation.
type CopyOptions struct {
	core.WriteContext
}
