// Package bulk provides batch filesystem operations.
package bulk

import (
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// Bulk provides batch operations.
type Bulk struct {
	docs *documents.Documents
}

// New creates a new Bulk instance.
func New(docs *documents.Documents) *Bulk {
	return &Bulk{docs: docs}
}
