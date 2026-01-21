// Package bulk provides batch filesystem operations.
package bulk

import (
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// Errors returned by bulk operations.
var (
	ErrImportFailed = errors.New("import failed")
	ErrExportFailed = errors.New("export failed")
	ErrPathNotDir   = errors.New("path is not a directory")
)

// Bulk provides batch operations.
type Bulk struct {
	docs *documents.Documents
}

// New creates a new Bulk instance.
func New(docs *documents.Documents) *Bulk {
	return &Bulk{docs: docs}
}
