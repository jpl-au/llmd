// Package events provides the internal event bus for store mutations.
//
// The bus connects packages that need to react to changes without direct
// dependencies. Handlers subscribe to events and are called synchronously
// in subscription order when an event is emitted. Subscribers include the
// FTS search index, the extension event bridge, the SSE hub, and the
// webhook dispatcher.
//
// Events are fire-and-forget: handlers observe after the fact and cannot
// block or veto the originating operation. Every handler receives every
// event; a handler that fails is logged and the next one still runs,
// because the change the event reports is already committed.
package events

import (
	"context"
	"log/slog"

	"github.com/jpl-au/llmd/pkg/events"
)

// Handler processes events from the event bus.
type Handler interface {
	HandleEvent(ctx context.Context, event events.Event) error
}

// Bus is a synchronous event bus for document operations.
// Events are processed in order by all handlers before returning.
type Bus struct {
	handlers []Handler
}

// New creates a new event bus.
func New() *Bus {
	return &Bus{}
}

// Subscribe adds a handler to receive events.
// Handlers are called in the order they are subscribed.
func (b *Bus) Subscribe(h Handler) {
	b.handlers = append(b.handlers, h)
}

// Emit sends an event to all subscribed handlers.
// Handlers are called synchronously in subscription order. A handler
// error is logged and does not stop delivery to the rest.
func (b *Bus) Emit(ctx context.Context, event events.Event) {
	for _, h := range b.handlers {
		if err := h.HandleEvent(ctx, event); err != nil {
			slog.Warn("event handler failed", "event", event.Type, "path", event.Path, "key", event.Key, "err", err)
		}
	}
}
