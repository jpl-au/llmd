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

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/events"
	"github.com/jpl-au/llmd/engine/resolve"
)

const namespace = "core:tag"

var (
	ErrNotFound = errors.New("there is no such tag.")
	ErrExists   = errors.New("the tag already exists.")
	ErrInvalid  = errors.New("that is not a valid tag name.")
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

// requireDocument returns documents.ErrNotFound or ErrDeleted unless a
// live document exists at path. Tags may only be attached to live
// documents.
func (t *Tags) requireDocument(ctx context.Context, path string) error {
	return t.docs.Check(ctx, path)
}
