package events

import (
	"context"
	"errors"
	"reflect"
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

// TestBus_Emit covers delivery across handler configurations. handlers
// gives the error each subscribed handler returns (nil for success) and
// wantEvents the number of events each must have received afterwards.
func TestBus_Emit(t *testing.T) {
	errHandler := errors.New("handler failed")

	tests := []struct {
		name       string
		handlers   []error
		event      events.Event
		wantErr    error
		wantEvents []int
	}{
		{
			name:       "single handler",
			handlers:   []error{nil},
			event:      events.Event{Type: events.DocumentWritten, Path: "test/doc", Key: "abc123"},
			wantEvents: []int{1},
		},
		{
			name:       "multiple handlers",
			handlers:   []error{nil, nil},
			event:      events.Event{Type: events.DocumentDeleted, Path: "test/doc"},
			wantEvents: []int{1, 1},
		},
		{
			// h2 must not receive the event once h1 fails.
			name:       "handler error stops delivery",
			handlers:   []error{errHandler, nil},
			event:      events.Event{Type: events.DocumentWritten, Path: "test"},
			wantErr:    errHandler,
			wantEvents: []int{0, 0},
		},
		{
			name:       "no handlers",
			handlers:   nil,
			event:      events.Event{Type: events.DocumentWritten, Path: "test"},
			wantEvents: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := New()
			var handlers []*mockHandler
			for _, err := range tt.handlers {
				h := &mockHandler{err: err}
				handlers = append(handlers, h)
				bus.Subscribe(h)
			}

			if err := bus.Emit(t.Context(), tt.event); err != tt.wantErr {
				t.Errorf("Emit() error = %v, want %v", err, tt.wantErr)
			}
			for i, h := range handlers {
				if len(h.events) != tt.wantEvents[i] {
					t.Errorf("handler %d received %d events, want %d", i, len(h.events), tt.wantEvents[i])
					continue
				}
				for _, got := range h.events {
					if !reflect.DeepEqual(got, tt.event) {
						t.Errorf("handler %d received %+v, want %+v", i, got, tt.event)
					}
				}
			}
		})
	}
}

// TestBus_HandlerOrder is a sequential scenario: handlers must be
// invoked in subscription order.
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
	if err := bus.Emit(t.Context(), event); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("expected order [1, 2], got %v", order)
	}
}

// handlerFunc wraps a function as a Handler
type handlerFunc func(ctx context.Context, event events.Event) error

func (f handlerFunc) HandleEvent(ctx context.Context, event events.Event) error {
	return f(ctx, event)
}
