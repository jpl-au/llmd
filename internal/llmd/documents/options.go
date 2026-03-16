package documents

import "github.com/jpl-au/llmd/pkg/model/core"

// WriteOptions configures a write operation.
type WriteOptions struct {
	core.Origin
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
	Sort           string // "time" for newest-first; default is by path
	SinceMS        int64  // Unix millis; 0 = no filter
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	core.Origin
}

// RestoreOptions configures a restore operation.
type RestoreOptions struct {
	core.Origin
}

// EditOptions configures an edit operation.
type EditOptions struct {
	core.Origin
	ReplaceAll bool // Replace all occurrences, not just first
}

// MoveOptions configures a move operation.
type MoveOptions struct {
	core.Origin
}

// CopyOptions configures a copy operation.
type CopyOptions struct {
	core.Origin
}
