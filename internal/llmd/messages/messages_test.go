package messages

import (
	"context"
	"testing"

	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/profile"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *qwr.Manager {
	t.Helper()
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	db, err := qwr.New("file::memory:?cache=shared").Reader(rp).Writer(wp).Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSendAndPending(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := context.Background()

	// Send a broadcast message.
	msg, err := store.Send(ctx, SendOptions{
		Type:   "document.written",
		Author: "human",
		Source: "cli",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.Key == "" {
		t.Fatal("expected non-empty key")
	}

	// Both consumers should see the broadcast.
	pending, err := store.Pending(ctx, "claude", 0)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("len(pending) = %d, want 1", len(pending))
	}
	if pending[0].Key != msg.Key {
		t.Errorf("key = %q, want %q", pending[0].Key, msg.Key)
	}
}

func TestDirectedMessage(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := context.Background()

	// Send directed to claude.
	_, err := store.Send(ctx, SendOptions{
		Type:       "direct",
		Author:     "human",
		Source:     "cli",
		AssignedTo: "claude",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Claude sees it.
	pending, err := store.Pending(ctx, "claude", 0)
	if err != nil {
		t.Fatalf("Pending (claude): %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("claude pending = %d, want 1", len(pending))
	}

	// Gemini does not.
	pending, err = store.Pending(ctx, "gemini", 0)
	if err != nil {
		t.Fatalf("Pending (gemini): %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("gemini pending = %d, want 0", len(pending))
	}
}

func TestAckOrdering(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := context.Background()

	// Send two messages.
	first, _ := store.Send(ctx, SendOptions{Type: "a", Author: "x", Source: "cli"})
	second, _ := store.Send(ctx, SendOptions{Type: "b", Author: "x", Source: "cli"})

	// Acking the second one first should fail.
	err := store.Ack(ctx, second.Key, "claude")
	if err == nil {
		t.Fatal("expected ErrOrderViolation, got nil")
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

func TestDeduplication(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := context.Background()

	// Send with a source_key.
	_, err := store.Send(ctx, SendOptions{
		Type:      "task.created",
		Author:    "human",
		Source:    "cli",
		SourceKey: "task.created:abc123",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Send again with the same source_key - should be silently ignored.
	_, err = store.Send(ctx, SendOptions{
		Type:      "task.created",
		Author:    "poller",
		Source:    "mcp",
		SourceKey: "task.created:abc123",
	})
	if err != nil {
		t.Fatalf("Send duplicate: %v", err)
	}

	// Only one message in the queue.
	pending, err := store.Pending(ctx, "claude", 0)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}
}
