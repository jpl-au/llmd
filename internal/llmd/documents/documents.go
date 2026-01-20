// Package documents provides document CRUD operations.
package documents

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound = errors.New("document not found")
	ErrDeleted  = errors.New("document is deleted")
)

// Documents provides document operations.
type Documents struct {
	db *sql.DB
}

// New creates a new Documents instance.
func New(db *sql.DB) *Documents {
	return &Documents{db: db}
}
