// Package tags provides document tagging operations.
package tags

import (
	"database/sql"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

const namespace = "core:tag"

var ErrNotFound = errors.New("tag not found")

// Tags provides tag operations.
type Tags struct {
	db   *sql.DB
	docs *documents.Documents
}

// New creates a new Tags instance.
func New(db *sql.DB, docs *documents.Documents) *Tags {
	return &Tags{db: db, docs: docs}
}
