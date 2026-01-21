// Package history provides document version operations.
package history

import (
	"database/sql"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

const namespace = "core:document"
const mime = "text/markdown"

// Errors returned by history operations.
var (
	ErrNotFound       = errors.New("document not found")
	ErrVersionInvalid = errors.New("invalid version")
)

// History provides version history operations.
type History struct {
	db   *sql.DB
	docs *documents.Documents
}

// New creates a new History instance.
func New(db *sql.DB, docs *documents.Documents) *History {
	return &History{db: db, docs: docs}
}
