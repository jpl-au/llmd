// Package search provides full-text and regex search operations.
package search

import (
	"database/sql"
	"errors"
)

var (
	ErrInvalidQuery   = errors.New("invalid FTS query")
	ErrInvalidPattern = errors.New("invalid regex pattern")
)

// Search provides search operations.
type Search struct {
	db *sql.DB
}

// New creates a new Search instance.
func New(db *sql.DB) *Search {
	return &Search{db: db}
}
