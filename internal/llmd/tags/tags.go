// Package tags provides document tagging operations.
//
// Tags are stored in the entities table with namespace "core:tag".
// The Relation field holds the document path being tagged.
// The Value field holds JSON with the tag name.
package tags

import (
	"errors"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/qwr"
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
}

// New creates a new Tags instance.
func New(db *qwr.Manager, docs *documents.Documents) *Tags {
	return &Tags{db: db, docs: docs}
}
