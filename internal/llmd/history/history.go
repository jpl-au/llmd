// Package history provides document version operations.
package history

import (
	"database/sql"
	"errors"
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
	db *sql.DB
}

// New creates a new History instance.
func New(db *sql.DB) *History {
	return &History{db: db}
}
