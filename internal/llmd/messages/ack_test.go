package messages

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/events"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// recorder captures the event types delivered on the bus.
type recorder struct {
	types []string
}

func (r *recorder) HandleEvent(_ context.Context, e pkgevents.Event) error {
	r.types = append(r.types, e.Type)
	return nil
}

// TestAck covers acknowledgement outcomes beyond the ordering flow in
// TestAckOrdering: the lookup errors, the unavailable work database,
// and the event a successful ack emits. ack indexes the keys setup
// returns; wantEvents lists the bus events emitted by the Ack itself.
func TestAck(t *testing.T) {
	send := func(t *testing.T, store *Messages, opts SendOptions) string {
		t.Helper()
		msg, err := store.Send(t.Context(), opts)
		if err != nil {
			t.Fatalf("Send(%+v) error = %v", opts, err)
		}
		return msg.Key
	}
	broadcast := SendOptions{Type: "a", Author: "human", Source: "cli"}

	tests := []struct {
		name       string
		noDB       bool // the work database cannot be opened
		setup      func(t *testing.T, store *Messages) []string
		ack        int
		key        string // used instead of ack when set
		consumer   string
		wantErr    error
		wantAnyErr bool
		wantEvents []string
	}{
		{
			name:       "acknowledges oldest and emits event",
			setup:      func(t *testing.T, store *Messages) []string { return []string{send(t, store, broadcast)} },
			consumer:   "claude",
			wantEvents: []string{pkgevents.MessageAcknowledged},
		},
		{
			name:     "empty queue",
			setup:    func(*testing.T, *Messages) []string { return nil },
			key:      "nokey0001",
			consumer: "claude",
			wantErr:  ErrNotFound,
		},
		{
			name:     "unknown key with a pending message",
			setup:    func(t *testing.T, store *Messages) []string { return []string{send(t, store, broadcast)} },
			key:      "nokey0001",
			consumer: "claude",
			wantErr:  ErrOrderViolation,
		},
		{
			name: "already acknowledged",
			setup: func(t *testing.T, store *Messages) []string {
				keys := []string{send(t, store, broadcast)}
				if err := store.Ack(t.Context(), keys[0], "claude"); err != nil {
					t.Fatalf("Ack(%s) error = %v", keys[0], err)
				}
				return keys
			},
			consumer: "claude",
			wantErr:  ErrNotFound,
		},
		{
			name: "directed to another consumer",
			setup: func(t *testing.T, store *Messages) []string {
				return []string{send(t, store, SendOptions{Type: "a", Author: "human", Source: "cli", AssignedTo: "gemini"})}
			},
			consumer: "claude",
			wantErr:  ErrNotFound,
		},
		{
			// Each consumer has its own cursor: gemini's ack does not
			// move claude's.
			name: "other consumer's ack does not count",
			setup: func(t *testing.T, store *Messages) []string {
				keys := []string{send(t, store, broadcast), send(t, store, broadcast)}
				if err := store.Ack(t.Context(), keys[0], "gemini"); err != nil {
					t.Fatalf("Ack(%s, gemini) error = %v", keys[0], err)
				}
				return keys
			},
			ack:      1,
			consumer: "claude",
			wantErr:  ErrOrderViolation,
		},
		{
			name:       "work database unavailable",
			noDB:       true,
			setup:      func(*testing.T, *Messages) []string { return nil },
			key:        "nokey0001",
			consumer:   "claude",
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			dbFn := func() *qwr.Manager { return db }
			if tt.noDB {
				dbFn = func() *qwr.Manager { return nil }
			}
			bus := events.New()
			rec := &recorder{}
			bus.Subscribe(rec)
			store := New(dbFn, bus)

			keys := tt.setup(t, store)
			rec.types = nil // only the events of the Ack under test

			key := tt.key
			if key == "" {
				key = keys[tt.ack]
			}

			err := store.Ack(t.Context(), key, tt.consumer)
			if tt.wantAnyErr {
				if err == nil {
					t.Fatalf("Ack(%s, %s) error = nil, want error", key, tt.consumer)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Ack(%s, %s) error = %v, want %v", key, tt.consumer, err, tt.wantErr)
			}
			if !slices.Equal(rec.types, tt.wantEvents) {
				t.Errorf("Ack(%s, %s) emitted %v, want %v", key, tt.consumer, rec.types, tt.wantEvents)
			}
			if tt.wantErr != nil {
				return
			}

			if _, err := store.Peek(t.Context(), tt.consumer); !errors.Is(err, ErrNotFound) {
				t.Errorf("Peek(%s) after Ack error = %v, want %v", tt.consumer, err, ErrNotFound)
			}
		})
	}
}
