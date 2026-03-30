package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// QueueHandler subscribes to the event bus and publishes domain events
// as queue messages. It skips queue's own events (message.sent,
// message.acknowledged) to avoid infinite loops. The ready function
// gates writes so document-only stores do not create work.db merely
// because the handler is subscribed.
type QueueHandler struct {
	store *Messages
	ready func() bool
}

// NewHandler creates a handler that bridges bus events to the queue.
// The ready function should return true only when work.db is open.
func NewHandler(store *Messages, ready func() bool) *QueueHandler {
	return &QueueHandler{store: store, ready: ready}
}

// HandleEvent converts a domain event into a queue message. The
// source_key is derived from the event type and entity key, so
// cross-process deduplication works automatically.
func (h *QueueHandler) HandleEvent(ctx context.Context, e pkgevents.Event) error {
	// Only queue events when work.db is open. Document-only
	// stores skip queue writes silently.
	if !h.ready() {
		return nil
	}

	// Skip queue's own events to avoid feedback loops.
	if strings.HasPrefix(e.Type, "message.") {
		return nil
	}

	sourceKey := e.Type
	if e.Key != "" {
		sourceKey += ":" + e.Key
	}

	payload, err := json.Marshal(e)
	if err != nil {
		slog.Warn("queue handler: marshal event", "type", e.Type, "err", err)
		return nil
	}

	_, err = h.store.Send(ctx, SendOptions{
		Type:       e.Type,
		Payload:    string(payload),
		Author:     e.Author,
		Source:     "bus",
		AssignedTo: e.AssignedTo,
		SourceKey:  sourceKey,
	})
	if err != nil {
		return fmt.Errorf("queue handler: %w", err)
	}
	return nil
}
