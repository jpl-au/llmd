// Package links provides document linking operations.
//
// Links are stored in the entities table with namespace "core:link".
// The Relation field holds the source document path (FROM).
// The Value field holds JSON with the target path and optional label.
//
// The package emits events via the event bus after mutations so
// cross-cutting concerns can react without coupling.
package links

import (
	"context"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
	"github.com/jpl-au/qwr"
)

const namespace = "core:link"

var (
	ErrExists   = errors.New("link already exists")
	ErrNotFound = errors.New("link not found")
	ErrSelfLink = errors.New("cannot link document to itself")
)

// Links provides link operations.
type Links struct {
	db   *qwr.Manager
	docs *documents.Documents
	bus  *events.Bus
}

// New creates a new Links instance.
func New(db *qwr.Manager, docs *documents.Documents, bus *events.Bus) *Links {
	return &Links{db: db, docs: docs, bus: bus}
}

// resolvePath translates a document identifier (path or key) to a path.
func (l *Links) resolvePath(ctx context.Context, value string) (string, error) {
	path, _, _ := resolve.Identifier(ctx, value, l.docs.KeyToPath)
	return path, nil
}
