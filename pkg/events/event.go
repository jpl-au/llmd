// Package events provides shared event types for the llmd document store.
package events

// Event represents a document store event.
type Event struct {
	// Type identifies the event (e.g., "document.written").
	Type string

	// Path is the document path that triggered the event.
	Path string

	// Key is the document key after the event.
	Key string

	// Version is the document version after the event.
	Version int

	// Author is the user or service that caused the event.
	Author string

	// Timestamp is the Unix timestamp (milliseconds) when the event occurred.
	Timestamp int64

	// Metadata contains additional event-specific information.
	Metadata map[string]any
}

// Event type constants.
const (
	// DocumentWritten fires when a document is created or updated.
	DocumentWritten = "document.written"

	// DocumentDeleted fires when a document is soft-deleted.
	DocumentDeleted = "document.deleted"

	// DocumentRestored fires when a soft-deleted document is restored.
	DocumentRestored = "document.restored"

	// DocumentMoved fires when a document is moved to a new path.
	DocumentMoved = "document.moved"
)
