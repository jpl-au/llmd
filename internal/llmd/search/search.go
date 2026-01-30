// Package search provides full-text search and path matching.
package search

import (
	"database/sql"
	"errors"
)

var (
	ErrInvalidQuery = errors.New("invalid FTS query")
	ErrInvalidGlob  = errors.New("invalid glob pattern")
)

// Search provides search operations.
type Search struct {
	db *sql.DB
}

// New creates a new Search instance.
func New(db *sql.DB) *Search {
	return &Search{db: db}
}
