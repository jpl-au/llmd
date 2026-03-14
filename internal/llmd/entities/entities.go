// Package entities provides generic entity operations for the entities table.
//
// The entities table is a general-purpose store for metadata, state, and relationships.
// It uses JSON in the Value field to provide unlimited flexibility. Higher-level
// concepts like tags and links are built on top of this foundation.
package entities

import (
	"errors"

	"github.com/jpl-au/qwr"
)

// Errors returned by entity operations.
var (
	ErrNotFound = errors.New("entity not found")
)

// Entities provides entity operations.
type Entities struct {
	db *qwr.Manager
}

// New creates a new Entities instance.
func New(db *qwr.Manager) *Entities {
	return &Entities{db: db}
}
