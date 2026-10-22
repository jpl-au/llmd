package messages

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/sqlite/sqlitetest"

	"github.com/jpl-au/qwr"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *qwr.Manager {
	t.Helper()
	db := sqlitetest.Open(t, "work.db")
	return db
}

// TestPending covers which sent messages a consumer sees as pending.
// want lists indices into sends whose keys must appear, in order.
func TestPending(t *testing.T) {
	tests := []struct {
		name     string
		sends    []SendOptions
		consumer string
		want     []int
	}{
		{
			name:     "broadcast seen by any consumer",
			sends:    []SendOptions{{Type: "document.written", Author: "human", Source: "cli"}},
			consumer: "claude",
			want:     []int{0},
		},
		{
			name:     "directed seen by assignee",
			sends:    []SendOptions{{Type: "direct", Author: "human", Source: "cli", AssignedTo: "claude"}},
			consumer: "claude",
			want:     []int{0},
		},
		{
			name:     "directed hidden from other consumer",
			sends:    []SendOptions{{Type: "direct", Author: "human", Source: "cli", AssignedTo: "claude"}},
			consumer: "gemini",
			want:     nil,
		},
		{
			// Sending again with the same source_key inserts nothing and
			// returns the message already stored under that key.
			name: "duplicate source key ignored",
			sends: []SendOptions{
				{Type: "task.created", Author: "human", Source: "cli", SourceKey: "task.created:abc123"},
				{Type: "task.created", Author: "poller", Source: "mcp", SourceKey: "task.created:abc123"},
			},
			consumer: "claude",
			want:     []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			store := New(func() *qwr.Manager { return db }, nil)

			var keys []string
			for i, opts := range tt.sends {
				msg, err := store.Send(t.Context(), opts)
				if err != nil {
					t.Fatalf("Send(%d) error = %v", i, err)
				}
				if msg.Key == "" {
					t.Fatalf("Send(%d) Key = %q, want non-empty", i, msg.Key)
				}
				keys = append(keys, msg.Key)
			}

			for i := range keys {
				for j := i + 1; j < len(keys); j++ {
					sk := tt.sends[i].SourceKey
					if sk != "" && sk == tt.sends[j].SourceKey && keys[i] != keys[j] {
						t.Errorf("Send(%d) Key = %q, want %q (same source key %q)", j, keys[j], keys[i], sk)
					}
				}
			}

			pending, err := store.Pending(t.Context(), tt.consumer, 0)
			if err != nil {
				t.Fatalf("Pending(%q) error = %v", tt.consumer, err)
			}
			if len(pending) != len(tt.want) {
				t.Fatalf("Pending(%q) returned %d messages, want %d", tt.consumer, len(pending), len(tt.want))
			}
			for i, idx := range tt.want {
				if pending[i].Key != keys[idx] {
					t.Errorf("Pending(%q)[%d].Key = %q, want %q", tt.consumer, i, pending[i].Key, keys[idx])
				}
			}
		})
	}
}

// TestAckOrdering is a sequential scenario: acknowledgements must be
// made oldest-first, and the queue drains once both are acknowledged.
func TestAckOrdering(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := t.Context()

	// Send two messages.
	first, err := store.Send(ctx, SendOptions{Type: "a", Author: "x", Source: "cli"})
	if err != nil {
		t.Fatalf("Send first: %v", err)
	}
	second, err := store.Send(ctx, SendOptions{Type: "b", Author: "x", Source: "cli"})
	if err != nil {
		t.Fatalf("Send second: %v", err)
	}

	// Acking the second one first should fail.
	if err := store.Ack(ctx, second.Key, "claude"); !errors.Is(err, ErrOrderViolation) {
		t.Fatalf("Ack(second) error = %v, want %v", err, ErrOrderViolation)
	}

	// Ack the first one - should succeed.
	if err := store.Ack(ctx, first.Key, "claude"); err != nil {
		t.Fatalf("Ack first: %v", err)
	}

	// Now ack the second one - should succeed.
	if err := store.Ack(ctx, second.Key, "claude"); err != nil {
		t.Fatalf("Ack second: %v", err)
	}

	// No more pending.
	pending, err := store.Pending(ctx, "claude", 0)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %d, want 0", len(pending))
	}
}
