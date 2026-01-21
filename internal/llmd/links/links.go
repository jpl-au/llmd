// Package links provides document linking operations.
//
// Links are stored in the entities table with namespace "core:link".
// The Relation field holds the source document path (FROM).
// The Value field holds JSON with the target path and optional label.
package links

import (
	"database/sql"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

const namespace = "core:link"

var (
	ErrExists   = errors.New("link already exists")
	ErrNotFound = errors.New("link not found")
	ErrSelfLink = errors.New("cannot link document to itself")
)

// Links provides link operations.
type Links struct {
	db   *sql.DB
	docs *documents.Documents
}

// New creates a new Links instance.
func New(db *sql.DB, docs *documents.Documents) *Links {
	return &Links{db: db, docs: docs}
}
