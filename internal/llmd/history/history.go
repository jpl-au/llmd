// Package history provides document version operations: listing version
// history, computing unified diffs between versions, and reverting to
// a previous version.
//
// History operates on the content table alongside the documents package.
// While documents handles CRUD, history handles read-only version
// navigation and the non-destructive revert operation (which creates a
// new version with old content rather than modifying existing rows).
package history

import (
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/qwr"
)

const namespace = "core:document"
const mime = "text/markdown"

// Errors returned by history operations.
var (
	ErrNotFound       = errors.New("document not found")
	ErrVersionInvalid = errors.New("invalid version")
)

// History provides version history operations against the content table.
// It reads version metadata (List), computes diffs between versions (Diff),
// and reverts to previous versions (Revert). All operations are read-only
// except Revert, which creates a new version via the Documents package.
type History struct {
	db   *qwr.Manager
	docs *documents.Documents
}

// New creates a History instance backed by the given database connection
// and Documents handle. The Documents handle is used by Revert to create
// new versions.
func New(db *qwr.Manager, docs *documents.Documents) *History {
	return &History{db: db, docs: docs}
}
