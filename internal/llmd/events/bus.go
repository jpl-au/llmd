// Package events provides the internal event bus for store mutations.
//
// The bus connects packages that need to react to changes without direct
// dependencies. Handlers subscribe to events and are called synchronously
// in subscription order when an event is emitted. Subscribers include the
// FTS search index, the extension event bridge, the SSE hub, and the
// webhook dispatcher.
//
// Events are fire-and-forget: handlers observe after the fact and cannot
// block or veto the originating operation. If a handler returns an error,
// event delivery stops and the error propagates to the caller.
package events

import (
	"context"

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
// Handlers are called synchronously in subscription order.
// Returns the first error encountered, if any.
func (b *Bus) Emit(ctx context.Context, event events.Event) error {
	for _, h := range b.handlers {
		if err := h.HandleEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
