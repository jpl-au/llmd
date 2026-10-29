package messages

import (
	"encoding/json"
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/events"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// queued describes one message the handler is expected to have
// written. event indexes the emitted event whose JSON is the payload.
type queued struct {
	SourceKey  string
	Type       string
	Author     string
	AssignedTo string
	event      int
}

// TestHandleEvent emits events on a real bus with the handler and the
// queue subscribed, as Store.wire does, then reads the queue back.
// Every Send emits message.sent on the same bus, so a case that queues
// anything also proves the handler ignores the queue's own events: a
// loop would recurse without end.
func TestHandleEvent(t *testing.T) {
	doc := pkgevents.Event{Type: pkgevents.DocumentWritten, Path: "docs/a", Key: "abc123", Version: 1, Author: "alice", Timestamp: 1_000}

	tests := []struct {
		name   string
		ready  bool
		noDB   bool // the work database cannot be opened
		events []pkgevents.Event
		want   []queued
	}{
		{
			name:   "document event queued",
			ready:  true,
			events: []pkgevents.Event{doc},
			want:   []queued{{SourceKey: "document.written:abc123", Type: pkgevents.DocumentWritten, Author: "alice"}},
		},
		{
			name:   "event without key uses type alone",
			ready:  true,
			events: []pkgevents.Event{{Type: pkgevents.TagAdded, Path: "docs/a", Author: "bob", Timestamp: 1_000}},
			want:   []queued{{SourceKey: "tag.added", Type: pkgevents.TagAdded, Author: "bob"}},
		},
		{
			name:   "directed event keeps assignee",
			ready:  true,
			events: []pkgevents.Event{{Type: pkgevents.TaskCreated, Key: "t1", Author: "alice", AssignedTo: "claude", Timestamp: 1_000}},
			want:   []queued{{SourceKey: "task.created:t1", Type: pkgevents.TaskCreated, Author: "alice", AssignedTo: "claude"}},
		},
		{
			name:  "message events skipped",
			ready: true,
			events: []pkgevents.Event{
				{Type: pkgevents.MessageSent, Key: "m1", Author: "alice", Timestamp: 1_000},
				{Type: pkgevents.MessageAcknowledged, Key: "m1", Author: "claude", Timestamp: 1_001},
			},
		},
		{
			name:   "not ready queues nothing",
			events: []pkgevents.Event{doc},
		},
		{
			// The first event wins; the repeat is dropped by source key
			// even though its author and payload differ.
			name:  "duplicate source key queues once",
			ready: true,
			events: []pkgevents.Event{
				doc,
				{Type: pkgevents.DocumentWritten, Path: "docs/a", Key: "abc123", Version: 1, Author: "bob", Timestamp: 2_000},
			},
			want: []queued{{SourceKey: "document.written:abc123", Type: pkgevents.DocumentWritten, Author: "alice"}},
		},
		{
			name:  "distinct keys queue separately",
			ready: true,
			events: []pkgevents.Event{
				doc,
				{Type: pkgevents.DocumentWritten, Path: "docs/b", Key: "def456", Version: 1, Author: "alice", Timestamp: 2_000},
			},
			want: []queued{
				{SourceKey: "document.written:abc123", Type: pkgevents.DocumentWritten, Author: "alice"},
				{SourceKey: "document.written:def456", Type: pkgevents.DocumentWritten, Author: "alice", event: 1},
			},
		},
		{
			name:   "work database unavailable",
			ready:  true,
			noDB:   true,
			events: []pkgevents.Event{doc},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			db := openTestDB(t)
			dbFn := func() *qwr.Manager { return db }
			if tt.noDB {
				dbFn = func() *qwr.Manager { return nil }
			}

			bus := events.New()
			store := New(dbFn, bus)
			h := NewHandler(store, func() bool { return tt.ready })

			for i, e := range tt.events {
				err := h.HandleEvent(ctx, e)
				if tt.noDB {
					if err == nil {
						t.Errorf("HandleEvent(%d) error = nil, want error when the work database is unavailable", i)
					}
					continue
				}
				if err != nil {
					t.Fatalf("HandleEvent(%d) error = %v", i, err)
				}
			}
			if tt.noDB {
				return
			}

			got, err := store.History(ctx, "", 0)
			if err != nil {
				t.Fatalf("History() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("History() returned %d messages, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, want := range tt.want {
				msg := got[i]
				if msg.SourceKey != want.SourceKey {
					t.Errorf("History()[%d].SourceKey = %q, want %q", i, msg.SourceKey, want.SourceKey)
				}
				if msg.Type != want.Type {
					t.Errorf("History()[%d].Type = %q, want %q", i, msg.Type, want.Type)
				}
				if msg.Author != want.Author {
					t.Errorf("History()[%d].Author = %q, want %q", i, msg.Author, want.Author)
				}
				if msg.AssignedTo != want.AssignedTo {
					t.Errorf("History()[%d].AssignedTo = %q, want %q", i, msg.AssignedTo, want.AssignedTo)
				}
				if msg.Source != "bus" {
					t.Errorf("History()[%d].Source = %q, want %q", i, msg.Source, "bus")
				}
				payload, err := json.Marshal(tt.events[want.event])
				if err != nil {
					t.Fatalf("marshalling event %d: %v", want.event, err)
				}
				if msg.Payload != string(payload) {
					t.Errorf("History()[%d].Payload = %s, want %s", i, msg.Payload, payload)
				}
			}
		})
	}
}
