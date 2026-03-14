// Package documents provides document CRUD operations against the content
// table in SQLite.
//
// Documents are versioned: every Write creates a new row with an
// incremented version number. Reads return the latest non-deleted version
// by default, or a specific version when requested. Deletes are soft:
// they set deleted_at on the latest version, making the document
// invisible to normal queries while preserving it for Restore.
//
// The package emits events via the event bus after mutations (writes,
// deletes, restores, moves) so cross-cutting concerns like the FTS
// search index can react without coupling.
package documents

import (
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

// Errors returned by document operations. ErrDeleted is returned
// alongside the document data — callers can inspect the returned
// document even when the error is ErrDeleted, which is useful for
// displaying metadata about soft-deleted items.
var (
	ErrNotFound = errors.New("document not found")
	ErrDeleted  = errors.New("document is deleted")
)

// Documents provides document operations against the content table.
// All methods take a context for cancellation and require a database
// connection and event bus, both provided at construction via [New].
type Documents struct {
	db  *qwr.Manager
	bus *events.Bus
}

// New creates a Documents instance backed by the given database and
// event bus. The bus receives events after successful mutations.
func New(db *qwr.Manager, bus *events.Bus) *Documents {
	return &Documents{db: db, bus: bus}
}
