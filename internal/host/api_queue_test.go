package host

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/messages"
	"github.com/jpl-au/llmd/sdk"
)

// sendMsg sends a queue message and fails the test on error.
func sendMsg(t *testing.T, c sdk.Context, opts sdk.SendOpts) *sdk.Message {
	t.Helper()
	if opts.Author == "" {
		opts.Author = "alice"
	}
	if opts.Type == "" {
		opts.Type = "direct"
	}
	m, err := c.Queue.Send(opts)
	if err != nil {
		t.Fatalf("Queue.Send(%+v) error = %v", opts, err)
	}
	return m
}

// TestQueuePending seeds a broadcast message, one directed to bob and
// one directed to carol, then checks what each consumer sees. Nothing
// else touches the store, so no domain events land in the queue.
func TestQueuePending(t *testing.T) {
	tests := []struct {
		name     string
		before   func(t *testing.T, c sdk.Context, keys []string)
		consumer string
		limit    int
		want     []string // payloads in order
	}{
		{name: "bob sees broadcast and his own", consumer: "bob", want: []string{"all", "for bob"}},
		{name: "carol sees broadcast and her own", consumer: "carol", want: []string{"all", "for carol"}},
		{name: "a stranger sees only broadcast", consumer: "dave", want: []string{"all"}},
		{name: "limit", consumer: "bob", limit: 1, want: []string{"all"}},
		{
			name: "acknowledged messages drop out",
			before: func(t *testing.T, c sdk.Context, keys []string) {
				if err := c.Queue.Ack(keys[0], "bob"); err != nil {
					t.Fatalf("Ack: %v", err)
				}
			},
			consumer: "bob",
			want:     []string{"for bob"},
		},
		{
			name: "acks are per consumer",
			before: func(t *testing.T, c sdk.Context, keys []string) {
				if err := c.Queue.Ack(keys[0], "bob"); err != nil {
					t.Fatalf("Ack: %v", err)
				}
			},
			consumer: "carol",
			want:     []string{"all", "for carol"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			keys := []string{
				sendMsg(t, c, sdk.SendOpts{Payload: "all"}).Key,
				sendMsg(t, c, sdk.SendOpts{Payload: "for bob", AssignedTo: "bob"}).Key,
				sendMsg(t, c, sdk.SendOpts{Payload: "for carol", AssignedTo: "carol"}).Key,
			}
			if tt.before != nil {
				tt.before(t, c, keys)
			}

			msgs, err := c.Queue.Pending(tt.consumer, tt.limit)
			if err != nil {
				t.Fatalf("Pending(%q, %d) error = %v", tt.consumer, tt.limit, err)
			}
			var got []string
			for _, m := range msgs {
				got = append(got, m.Payload)
				if m.Key == "" || m.Author != "alice" || m.Type != "direct" || m.CreatedAt == 0 {
					t.Errorf("Pending(%q, %d) message = %+v, want key, author alice, type direct and a timestamp", tt.consumer, tt.limit, m)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Pending(%q, %d) = %v, want %v", tt.consumer, tt.limit, got, tt.want)
			}
		})
	}
}

// TestQueueErr covers the translation of every messages sentinel.
func TestQueueErr(t *testing.T) {
	other := errors.New("disk on fire")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"not found", messages.ErrNotFound, sdk.ErrNotFound},
		{"order violation", messages.ErrOrderViolation, sdk.ErrOrderViolation},
		{"passthrough", other, other},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queueErr(tt.in)
			if !errors.Is(got, tt.want) {
				t.Errorf("queueErr(%v) = %v, want %v", tt.in, got, tt.want)
			}
			if tt.in != nil && !errors.Is(got, tt.in) {
				t.Errorf("queueErr(%v) = %v, want to wrap the original", tt.in, got)
			}
		})
	}
}

// TestQueueAckOrder checks the sentinel reaches the SDK for an
// out-of-order acknowledgement.
func TestQueueAckOrder(t *testing.T) {
	c := testHost(t)
	first := sendMsg(t, c, sdk.SendOpts{Payload: "first"})
	second := sendMsg(t, c, sdk.SendOpts{Payload: "second"})

	if err := c.Queue.Ack(second.Key, "bob"); !errors.Is(err, sdk.ErrOrderViolation) {
		t.Errorf("Ack(second) error = %v, want %v", err, sdk.ErrOrderViolation)
	}
	if err := c.Queue.Ack(first.Key, "bob"); err != nil {
		t.Errorf("Ack(first) error = %v, want nil", err)
	}
	if err := c.Queue.Ack(second.Key, "bob"); err != nil {
		t.Errorf("Ack(second) after first error = %v, want nil", err)
	}
}
