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

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
)

const namespace = "core:link"

var (
	ErrExists   = errors.New("the link already exists.")
	ErrNotFound = errors.New("there is no such link.")
	ErrSelfLink = errors.New("a document cannot link to itself.")
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
func (l *Links) resolvePath(ctx context.Context, value string) string {
	path, _, _ := resolve.Identifier(ctx, value, l.docs.KeyToPath)
	return path
}

// requireDocument returns documents.ErrNotFound or ErrDeleted unless a
// live document exists at path. Links may only connect live documents.
func (l *Links) requireDocument(ctx context.Context, path string) error {
	return l.docs.Check(ctx, path)
}
