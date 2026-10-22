// Package bulk provides batch import and export between the document
// store and the filesystem.
//
// Import reads markdown files from a directory into the store, detecting
// whether each file is new, updated, or unchanged (via XXH3 hash
// comparison). Export writes store documents back to the filesystem as
// .md files, preserving path hierarchy. Both operations support prefix
// filtering, dry-run mode, and force overwrite.
package bulk

import (
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// Errors returned by bulk operations.
var ()

// Bulk provides batch import and export operations. It delegates
// individual document reads and writes to the Documents package,
// handling the filesystem traversal and change-detection logic.
type Bulk struct {
	docs *documents.Documents
}

// New creates a Bulk instance that delegates document operations to
// the given Documents handle.
func New(docs *documents.Documents) *Bulk {
	return &Bulk{docs: docs}
}
