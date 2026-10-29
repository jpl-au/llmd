// Package documents provides document CRUD operations against the content
// table in SQLite.
//
// Documents are versioned: every Write creates a new row with an
// incremented version number. Reads return the latest non-deleted version
// by default, or a specific version when requested. Deletes are soft:
// they set deleted_at on every version, making the document invisible
// to normal queries while preserving it for Restore.
//
// The package emits events via the event bus after mutations (writes,
// deletes, restores, moves) so cross-cutting concerns like the FTS
// search index can react without coupling.
package documents

import (
	"errors"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/events"
)

// Errors returned by document operations. ErrDeleted is returned
// alongside the document data - callers can inspect the returned
// document even when the error is ErrDeleted, which is useful for
// displaying metadata about soft-deleted items.
//
// A document is either live or deleted as a whole: every version of a
// path shares one state. Delete and Restore change all versions, and a
// write to a deleted path brings its earlier versions back with it, so
// history never has hidden parts.
var (
	ErrNotFound   = errors.New("the document does not exist.")
	ErrDeleted    = errors.New("the document is deleted. Restore brings it back.")
	ErrExists     = errors.New("the destination already exists.")
	ErrNotDeleted = errors.New("the document is not deleted.")
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
