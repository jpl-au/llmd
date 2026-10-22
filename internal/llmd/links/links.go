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
	"fmt"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
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
func (l *Links) resolvePath(ctx context.Context, value string) string {
	path, _, _ := resolve.Identifier(ctx, value, l.docs.KeyToPath)
	return path
}

// requireDocument returns documents.ErrNotFound unless a live document
// exists at path. Links may only connect documents that exist.
func (l *Links) requireDocument(ctx context.Context, path string) error {
	exists, err := l.docs.Exists(ctx, path)
	if err != nil {
		return fmt.Errorf("checking document: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s: %w", path, documents.ErrNotFound)
	}
	return nil
}
