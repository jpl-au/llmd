package sdk

import "errors"

// ErrOrderViolation means the consumer tried to acknowledge a message
// that is not their oldest pending message. The queue is strictly
// ordered — consumers must process front to back.
var ErrOrderViolation = errors.New("must acknowledge oldest message first")

// QueueStore is the message queue interface.
//
// The queue is a strictly ordered, durable notification layer. Domain
// events (document writes, task moves, audit creation) and direct
// messages from humans or agents all land here. Consumers poll the
// queue, process messages in order, and acknowledge each one.
//
// Messages are insert-only. Acks are insert-only. Consumers are
// identified by their --author identity.
type QueueStore interface {
	// Send inserts a message into the queue. Used by humans and agents
	// for direct messages, and by the event bus subscriber for domain
	// events.
	Send(opts SendOpts) (*Message, error)

	// Pending returns unacknowledged messages for the consumer, ordered
	// by created_at ascending. Limit controls batch size; zero means all.
	Pending(consumer string, limit int) ([]Message, error)

	// Peek returns the oldest unacknowledged message for the consumer.
	// Returns ErrNotFound when the queue is empty for this consumer.
	Peek(consumer string) (*Message, error)

	// Ack marks a message as acknowledged by the consumer. The key must
	// be the consumer's oldest pending message; otherwise Ack returns
	// ErrOrderViolation.
	Ack(key, consumer string) error

	// History returns all messages (including acknowledged) with optional
	// filters.
	History(opts HistoryOpts) ([]Message, error)
}

// Message is the SDK view of a queue message.
type Message struct {
	// Key is the stable 9-character identifier.
	Key string

	// Type identifies the event (e.g. "document.written", "direct").
	Type string

	// Payload is the JSON event data.
	Payload string

	// Author is who caused the event or sent the message.
	Author string

	// Source is where the message originated ("cli", "mcp", "http").
	Source string

	// AssignedTo is the intended recipient. Empty means broadcast.
	AssignedTo string

	// CreatedAt is the Unix timestamp (milliseconds).
	CreatedAt int64
}

// SendOpts configures a queue send operation.
type SendOpts struct {
	// Type is the event type (e.g. "document.written", "direct").
	Type string

	// Payload is the JSON event data.
	Payload string

	// Author is who is sending. Required.
	Author string

	// Source is where the message originates ("cli", "mcp", "http").
	Source string

	// AssignedTo directs the message to a specific consumer.
	// Empty means broadcast to all consumers.
	AssignedTo string

	// SourceKey is the deduplication key. When non-empty, a second
	// insert with the same SourceKey is silently ignored.
	SourceKey string
}

// HistoryOpts filters the queue history.
type HistoryOpts struct {
	// Consumer filters to messages visible to this consumer.
	// Empty means all messages.
	Consumer string

	// Since filters to messages created after this time (unix millis).
	// Zero means no filter.
	Since int64
}
