//go:build wasip1

package sdk

// Event represents a document store event.
//
// Events are delivered to plugins that implement EventHandler and have
// subscribed to the event type in their Manifest.
type Event struct {
	// Type identifies the event (e.g., "document.written").
	Type string `json:"type"`

	// Path is the document path that triggered the event.
	Path string `json:"path"`

	// Version is the document version after the event.
	Version int `json:"version,omitempty"`

	// Author is the user or service that caused the event.
	Author string `json:"author"`

	// Timestamp is the Unix timestamp (nanoseconds) when the event occurred.
	Timestamp int64 `json:"timestamp"`

	// Metadata contains additional event-specific information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Event type constants for subscription.
//
// Use these constants when declaring SubscribedEvents in your Manifest
// and when comparing Event.Type in your EventHandler.
const (
	// EventDocumentWritten fires when a document is created or updated.
	EventDocumentWritten = "document.written"

	// EventDocumentDeleted fires when a document is soft-deleted.
	EventDocumentDeleted = "document.deleted"

	// EventDocumentMoved fires when a document is moved to a new path.
	EventDocumentMoved = "document.moved"

	// EventDocumentRestored fires when a soft-deleted document is restored.
	EventDocumentRestored = "document.restored"

	// EventTagAdded fires when a tag is added to a document.
	EventTagAdded = "tag.added"

	// EventTagRemoved fires when a tag is removed from a document.
	EventTagRemoved = "tag.removed"

	// EventLinkAdded fires when a link is created between documents.
	EventLinkAdded = "link.added"

	// EventLinkRemoved fires when a link is removed between documents.
	EventLinkRemoved = "link.removed"
)
