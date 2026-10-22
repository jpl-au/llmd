// Package tags provides document tagging operations.
//
// Tags are stored in the entities table with namespace "core:tag".
// The Relation field holds the document path being tagged.
// The Value field holds JSON with the tag name.
//
// The package emits events via the event bus after mutations so
// cross-cutting concerns can react without coupling.
package tags

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
)

const namespace = "core:tag"

var (
	ErrNotFound = errors.New("tag not found")
	ErrExists   = errors.New("tag already exists")
	ErrInvalid  = errors.New("invalid tag name")
)

// Tags provides tag operations.
type Tags struct {
	db   *qwr.Manager
	docs *documents.Documents
	bus  *events.Bus
}

// New creates a new Tags instance.
func New(db *qwr.Manager, docs *documents.Documents, bus *events.Bus) *Tags {
	return &Tags{db: db, docs: docs, bus: bus}
}

// resolvePath translates a document identifier (path or key) to a path.
func (t *Tags) resolvePath(ctx context.Context, value string) string {
	path, _, _ := resolve.Identifier(ctx, value, t.docs.KeyToPath)
	return path
}

// requireDocument returns documents.ErrNotFound unless a live document
// exists at path. Tags may only be attached to documents that exist.
func (t *Tags) requireDocument(ctx context.Context, path string) error {
	exists, err := t.docs.Exists(ctx, path)
	if err != nil {
		return fmt.Errorf("checking document: %w", err)
	}
	if !exists {
		return fmt.Errorf("%s: %w", path, documents.ErrNotFound)
	}
	return nil
}
