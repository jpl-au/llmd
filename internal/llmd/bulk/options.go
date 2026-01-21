package bulk

import "github.com/jpl-au/llmd/internal/llmd/core"

// ImportOptions configures an import operation.
type ImportOptions struct {
	core.WriteContext
	Prefix  string // Target path prefix in store
	Flatten bool   // Flatten directory structure
	Hidden  bool   // Include hidden files/directories
	DryRun  bool   // Show what would be imported without importing
	Force   bool   // Import even if content is unchanged
}

// ExportOptions configures an export operation.
type ExportOptions struct {
	Overwrite bool // Overwrite existing files
	Version   *int // Export specific version (for single doc)
}
