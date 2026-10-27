package messages

import (
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/key"
)

// TestSendTakenKey covers a key clash between processes: the second
// send is stored under a fresh key and both messages survive.
func TestSendTakenKey(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := t.Context()
	t.Cleanup(func() { key.Force() })

	key.Force("zzzzzzzzz1")
	first, err := store.Send(ctx, SendOptions{Type: "note", Payload: "one", Author: "a", Source: "test"})
	if err != nil {
		t.Fatalf("Send(one): %v", err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	second, err := store.Send(ctx, SendOptions{Type: "note", Payload: "two", Author: "a", Source: "test"})
	if err != nil {
		t.Fatalf("Send(two) with a taken key: %v", err)
	}
	if first.Key != "zzzzzzzzz1" || second.Key != "zzzzzzzzz2" {
		t.Errorf("keys = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", first.Key, second.Key)
	}
	msgs, err := store.Pending(ctx, "reader", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("pending = %d messages, want both", len(msgs))
	}
}

// TestSendDuplicateSourceKey covers the same event queued twice: the
// second send returns the message already stored.
func TestSendDuplicateSourceKey(t *testing.T) {
	db := openTestDB(t)
	store := New(func() *qwr.Manager { return db }, nil)
	ctx := t.Context()

	first, err := store.Send(ctx, SendOptions{Type: "task.created", Payload: "one", Author: "a", Source: "bus", SourceKey: "task.created:k1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Send(ctx, SendOptions{Type: "task.created", Payload: "two", Author: "a", Source: "bus", SourceKey: "task.created:k1"})
	if err != nil {
		t.Fatalf("Send(duplicate source key): %v", err)
	}
	if second.Key != first.Key || second.Payload != "one" {
		t.Errorf("duplicate send = %+v, want the first message %+v", second, first)
	}
	msgs, err := store.Pending(ctx, "reader", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Errorf("pending = %d messages, want one", len(msgs))
	}
}
