package events

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/pkg/events"
)

type mockHandler struct {
	events []events.Event
	err    error
}

func (h *mockHandler) HandleEvent(ctx context.Context, event events.Event) error {
	if h.err != nil {
		return h.err
	}
	h.events = append(h.events, event)
	return nil
}

func TestBus_Emit(t *testing.T) {
	bus := New()
	h := &mockHandler{}
	bus.Subscribe(h)

	event := events.Event{
		Type: events.DocumentWritten,
		Path: "test/doc",
		Key:  "abc123",
	}

	err := bus.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(h.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(h.events))
	}
	if h.events[0].Type != events.DocumentWritten {
		t.Errorf("expected type %s, got %s", events.DocumentWritten, h.events[0].Type)
	}
	if h.events[0].Path != "test/doc" {
		t.Errorf("expected path test/doc, got %s", h.events[0].Path)
	}
}

func TestBus_MultipleHandlers(t *testing.T) {
	bus := New()
	h1 := &mockHandler{}
	h2 := &mockHandler{}
	bus.Subscribe(h1)
	bus.Subscribe(h2)

	event := events.Event{
		Type: events.DocumentDeleted,
		Path: "test/doc",
	}

	err := bus.Emit(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(h1.events) != 1 {
		t.Errorf("h1: expected 1 event, got %d", len(h1.events))
	}
	if len(h2.events) != 1 {
		t.Errorf("h2: expected 1 event, got %d", len(h2.events))
	}
}

func TestBus_HandlerOrder(t *testing.T) {
	bus := New()
	var order []int

	h1 := &mockHandler{}
	h2 := &mockHandler{}

	// Wrap handlers to track order
	bus.Subscribe(handlerFunc(func(ctx context.Context, e events.Event) error {
		order = append(order, 1)
		return h1.HandleEvent(ctx, e)
	}))
	bus.Subscribe(handlerFunc(func(ctx context.Context, e events.Event) error {
		order = append(order, 2)
		return h2.HandleEvent(ctx, e)
	}))

	event := events.Event{Type: events.DocumentWritten, Path: "test"}
	if err := bus.Emit(context.Background(), event); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("expected order [1, 2], got %v", order)
	}
}

func TestBus_HandlerError(t *testing.T) {
	bus := New()
	expectedErr := errors.New("handler failed")

	h1 := &mockHandler{err: expectedErr}
	h2 := &mockHandler{}

	bus.Subscribe(h1)
	bus.Subscribe(h2)

	event := events.Event{Type: events.DocumentWritten, Path: "test"}
	err := bus.Emit(context.Background(), event)

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	// h2 should not have received the event due to h1's error
	if len(h2.events) != 0 {
		t.Errorf("h2 should not have received events, got %d", len(h2.events))
	}
}

func TestBus_NoHandlers(t *testing.T) {
	bus := New()
	event := events.Event{Type: events.DocumentWritten, Path: "test"}

	err := bus.Emit(context.Background(), event)
	if err != nil {
		t.Errorf("unexpected error with no handlers: %v", err)
	}
}

// handlerFunc wraps a function as a Handler
type handlerFunc func(ctx context.Context, event events.Event) error

func (f handlerFunc) HandleEvent(ctx context.Context, event events.Event) error {
	return f(ctx, event)
}
