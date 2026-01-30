// Package events provides the internal event bus for document operations.
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
