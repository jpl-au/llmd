// Package message provides the Message and Ack models for the queue.
//
// Messages are insert-only records in the queue. Each message represents
// either a domain event (document written, task moved) or a direct
// human/agent communication. Acks track which consumer has processed
// which message, enabling per-consumer cursors through the ordered queue.
package message

import "github.com/jpl-au/llmd/pkg/model/core"

// Message represents a queue entry. Messages are immutable once inserted.
// The queue is strictly ordered by CreatedAt — consumers must process
// messages front to back.
type Message struct {
	// ID is the auto-increment database row ID.
	ID int64

	// Key is the stable 9-character base36 identifier.
	Key string

	// SourceKey is the deduplication key, derived from the domain event
	// as "event_type:entity_key". NULL for direct messages (no domain
	// event to deduplicate against).
	SourceKey string

	// Type identifies the event (e.g. "document.written", "task.moved",
	// "direct" for human-sent messages).
	Type string

	// Payload is the JSON-serialised event data.
	Payload string

	// AssignedTo is the intended recipient. Empty means broadcast
	// (all consumers see the message).
	AssignedTo string

	// Origin embeds authorship and provenance (Author, Source).
	core.Origin

	// CreatedAt is the Unix timestamp (milliseconds) when inserted.
	CreatedAt int64
}

// Ack records that a consumer has processed a message.
type Ack struct {
	// ID is the auto-increment database row ID.
	ID int64

	// MessageKey references the acknowledged message.
	MessageKey string

	// Author is the consumer who acknowledged.
	Author string

	// CreatedAt is the Unix timestamp (milliseconds) when acknowledged.
	CreatedAt int64
}
