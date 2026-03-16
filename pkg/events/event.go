// Package events provides shared event types for the llmd store.
//
// Events are emitted after successful mutations across all domains
// (documents, tags, links, audits, tasks) and consumed by handlers
// like the FTS search index and extension event bridge. The Event
// struct is shared between the internal event bus and any external
// subscribers.
//
// Event types follow a "domain.action" naming convention (e.g.
// "document.written", "audit.created"). The Metadata map carries
// event-specific data such as old_path for move events or tag
// names for tag events.
package events

// Event represents a store event. Fields are populated as
// appropriate for the domain — not every field applies to every
// event type. Path holds the document path for document/tag/link
// events and the target path for audit events. Key holds the
// entity key (document key, audit ID, task key, etc.).
type Event struct {
	// Type identifies the event (e.g., "document.written").
	Type string `json:"type"`

	// Path is the document or target path related to the event.
	Path string `json:"path,omitempty"`

	// Key is the entity key (document key, audit ID, task key, etc.).
	Key string `json:"key,omitempty"`

	// Version is the document version after the event (documents only).
	Version int `json:"version,omitempty"`

	// Author is the user or service that caused the event.
	Author string `json:"author,omitempty"`

	// Timestamp is the Unix timestamp (milliseconds) when the event occurred.
	Timestamp int64 `json:"timestamp"`

	// Metadata contains additional event-specific information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Event type constants follow "domain.action" naming.
const (
	// DocumentWritten fires when a document is created or updated.
	DocumentWritten = "document.written"

	// DocumentDeleted fires when a document is soft-deleted.
	DocumentDeleted = "document.deleted"

	// DocumentRestored fires when a soft-deleted document is restored.
	DocumentRestored = "document.restored"

	// DocumentMoved fires when a document is moved to a new path.
	DocumentMoved = "document.moved"

	// TagAdded fires when a tag is added to a document.
	TagAdded = "tag.added"

	// TagRemoved fires when a tag is removed from a document.
	TagRemoved = "tag.removed"

	// LinkCreated fires when a link between documents is created.
	LinkCreated = "link.created"

	// LinkRemoved fires when a link between documents is removed.
	LinkRemoved = "link.removed"

	// AuditCreated fires when a new audit thread is created.
	AuditCreated = "audit.created"

	// AuditReplied fires when a reply is added to an audit thread.
	AuditReplied = "audit.replied"

	// AuditResolved fires when an audit is marked as approved.
	AuditResolved = "audit.resolved"

	// AuditDeleted fires when an audit is soft-deleted.
	AuditDeleted = "audit.deleted"

	// AuditRestored fires when a soft-deleted audit is restored.
	AuditRestored = "audit.restored"

	// TaskCreated fires when a new task is created.
	TaskCreated = "task.created"

	// TaskMoved fires when a task moves between columns.
	TaskMoved = "task.moved"

	// TaskUpdated fires when task metadata is changed.
	TaskUpdated = "task.updated"

	// TaskDeleted fires when a task is soft-deleted.
	TaskDeleted = "task.deleted"

	// TaskRestored fires when a soft-deleted task is restored.
	TaskRestored = "task.restored"
)
