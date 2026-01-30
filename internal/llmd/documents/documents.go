// Package documents provides document CRUD operations.
package documents

import (
	"database/sql"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/events"
)

var (
	ErrNotFound = errors.New("document not found")
	ErrDeleted  = errors.New("document is deleted")
)

// Documents provides document operations.
type Documents struct {
	db  *sql.DB
	bus *events.Bus
}

// New creates a new Documents instance.
func New(db *sql.DB, bus *events.Bus) *Documents {
	return &Documents{db: db, bus: bus}
}
