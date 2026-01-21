package history

import "github.com/jpl-au/llmd/internal/llmd/core"

// ListOptions configures a list operation.
type ListOptions struct {
	Limit          int  // Max versions to return (0 = no limit)
	IncludeDeleted bool // Include deleted versions
}

// RevertOptions configures a revert operation.
type RevertOptions struct {
	core.WriteContext
}

// DiffOptions configures a diff operation.
type DiffOptions struct {
	Context int // lines of context around changes (0 = default)
}
