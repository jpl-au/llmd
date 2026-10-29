package messages

import (
	"errors"
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/clock/clocktest"
)

// seedHistory sends three messages at 1000, 2000 and 3000 ms (the
// second directed to gemini) and has claude acknowledge the first.
// It returns the keys in send order.
func seedHistory(t *testing.T, store *Messages) []string {
	t.Helper()
	ctx := t.Context()
	clocktest.Sequence(t, 1_000, 1_000)

	sends := []SendOptions{
		{Type: "a", Author: "human", Source: "cli"},
		{Type: "b", Author: "human", Source: "cli", AssignedTo: "gemini"},
		{Type: "c", Author: "human", Source: "cli"},
	}
	var keys []string
	for i, opts := range sends {
		msg, err := store.Send(ctx, opts)
		if err != nil {
			t.Fatalf("Send(%d) error = %v", i, err)
		}
		keys = append(keys, msg.Key)
	}
	if err := store.Ack(ctx, keys[0], "claude"); err != nil {
		t.Fatalf("Ack(%s, claude) error = %v", keys[0], err)
	}
	return keys
}

// TestHistory covers the consumer and since filters over the seeded
// queue. Unlike Pending, acknowledged messages stay in the history.
// want lists indices into the seeded sends, in order.
func TestHistory(t *testing.T) {
	tests := []struct {
		name     string
		consumer string
		since    int64
		want     []int
	}{
		{
			name: "all messages including acknowledged",
			want: []int{0, 1, 2},
		},
		{
			name:     "consumer filter hides messages directed elsewhere",
			consumer: "claude",
			want:     []int{0, 2},
		},
		{
			name:     "consumer filter keeps own directed messages",
			consumer: "gemini",
			want:     []int{0, 1, 2},
		},
		{
			name:  "since is exclusive",
			since: 1_000,
			want:  []int{1, 2},
		},
		{
			name:  "since between messages",
			since: 2_500,
			want:  []int{2},
		},
		{
			name:  "since after the newest",
			since: 3_000,
			want:  nil,
		},
		{
			name:     "consumer and since combined",
			consumer: "claude",
			since:    1_000,
			want:     []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			store := New(func() *qwr.Manager { return db }, nil)
			keys := seedHistory(t, store)

			got, err := store.History(t.Context(), tt.consumer, tt.since)
			if err != nil {
				t.Fatalf("History(%q, %d) error = %v", tt.consumer, tt.since, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("History(%q, %d) returned %d messages, want %d", tt.consumer, tt.since, len(got), len(tt.want))
			}
			for i, idx := range tt.want {
				if got[i].Key != keys[idx] {
					t.Errorf("History(%q, %d)[%d].Key = %q, want %q", tt.consumer, tt.since, i, got[i].Key, keys[idx])
				}
				if want := int64(idx+1) * 1_000; got[i].CreatedAt != want {
					t.Errorf("History(%q, %d)[%d].CreatedAt = %d, want %d", tt.consumer, tt.since, i, got[i].CreatedAt, want)
				}
			}
		})
	}
}

// TestPeek covers which message, if any, is at the front of a
// consumer's queue. want indexes the keys setup returns; -1 means
// ErrNotFound.
func TestPeek(t *testing.T) {
	send := func(t *testing.T, store *Messages, opts SendOptions) string {
		t.Helper()
		msg, err := store.Send(t.Context(), opts)
		if err != nil {
			t.Fatalf("Send(%+v) error = %v", opts, err)
		}
		return msg.Key
	}

	tests := []struct {
		name     string
		setup    func(t *testing.T, store *Messages) []string
		consumer string
		want     int
		wantErr  error
	}{
		{
			name:     "empty queue",
			setup:    func(*testing.T, *Messages) []string { return nil },
			consumer: "claude",
			want:     -1,
			wantErr:  ErrNotFound,
		},
		{
			name: "oldest pending",
			setup: func(t *testing.T, store *Messages) []string {
				return []string{
					send(t, store, SendOptions{Type: "a", Author: "human", Source: "cli"}),
					send(t, store, SendOptions{Type: "b", Author: "human", Source: "cli"}),
				}
			},
			consumer: "claude",
			want:     0,
		},
		{
			name: "advances past acknowledged",
			setup: func(t *testing.T, store *Messages) []string {
				keys := []string{
					send(t, store, SendOptions{Type: "a", Author: "human", Source: "cli"}),
					send(t, store, SendOptions{Type: "b", Author: "human", Source: "cli"}),
				}
				if err := store.Ack(t.Context(), keys[0], "claude"); err != nil {
					t.Fatalf("Ack(%s) error = %v", keys[0], err)
				}
				return keys
			},
			consumer: "claude",
			want:     1,
		},
		{
			name: "directed to another consumer is hidden",
			setup: func(t *testing.T, store *Messages) []string {
				return []string{send(t, store, SendOptions{Type: "a", Author: "human", Source: "cli", AssignedTo: "gemini"})}
			},
			consumer: "claude",
			want:     -1,
			wantErr:  ErrNotFound,
		},
		{
			name: "all acknowledged",
			setup: func(t *testing.T, store *Messages) []string {
				keys := []string{send(t, store, SendOptions{Type: "a", Author: "human", Source: "cli"})}
				if err := store.Ack(t.Context(), keys[0], "claude"); err != nil {
					t.Fatalf("Ack(%s) error = %v", keys[0], err)
				}
				return keys
			},
			consumer: "claude",
			want:     -1,
			wantErr:  ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			store := New(func() *qwr.Manager { return db }, nil)
			clocktest.Sequence(t, 1_000, 1)
			keys := tt.setup(t, store)

			got, err := store.Peek(t.Context(), tt.consumer)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Peek(%q) error = %v, want %v", tt.consumer, err, tt.wantErr)
			}
			if tt.want < 0 {
				if got != nil {
					t.Errorf("Peek(%q) = %+v, want nil", tt.consumer, got)
				}
				return
			}
			if got == nil || got.Key != keys[tt.want] {
				t.Errorf("Peek(%q) = %+v, want Key %q", tt.consumer, got, keys[tt.want])
			}
		})
	}
}

// TestBySourceKey covers the lookup Send falls back to when a source
// key is already stored.
func TestBySourceKey(t *testing.T) {
	tests := []struct {
		name      string
		seed      string // source key to send first, if any
		lookup    string
		wantFound bool
		wantErr   error
	}{
		{
			name:      "existing",
			seed:      "task.created:abc123",
			lookup:    "task.created:abc123",
			wantFound: true,
		},
		{
			name:    "missing",
			seed:    "task.created:abc123",
			lookup:  "task.created:zzz999",
			wantErr: ErrNotFound,
		},
		{
			name:    "empty queue",
			lookup:  "task.created:abc123",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			store := New(func() *qwr.Manager { return db }, nil)
			if err := store.ensure(); err != nil {
				t.Fatalf("ensure() error = %v", err)
			}

			var seeded string
			if tt.seed != "" {
				msg, err := store.Send(t.Context(), SendOptions{Type: "task.created", Author: "human", Source: "cli", SourceKey: tt.seed})
				if err != nil {
					t.Fatalf("Send() error = %v", err)
				}
				seeded = msg.Key
			}

			got, err := store.bySourceKey(t.Context(), tt.lookup)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("bySourceKey(%q) error = %v, want %v", tt.lookup, err, tt.wantErr)
			}
			if !tt.wantFound {
				if got != nil {
					t.Errorf("bySourceKey(%q) = %+v, want nil", tt.lookup, got)
				}
				return
			}
			if got == nil || got.Key != seeded || got.SourceKey != tt.lookup {
				t.Errorf("bySourceKey(%q) = %+v, want Key %q SourceKey %q", tt.lookup, got, seeded, tt.lookup)
			}
		})
	}
}
