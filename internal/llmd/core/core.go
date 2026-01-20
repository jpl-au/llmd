// Package core provides shared types for llmd operations.
package core

// WriteContext contains common fields for write operations.
// Embed this in options structs that modify data.
type WriteContext struct {
	Author  string // Required: who is making this change
	Source  string // Required: cli, mcp, import, sync, api
	Message string // Optional: commit-style message
}

// Validate checks that required fields are set.
func (w WriteContext) Validate() error {
	if w.Author == "" {
		return ErrAuthorRequired
	}
	if w.Source == "" {
		return ErrSourceRequired
	}
	return nil
}
