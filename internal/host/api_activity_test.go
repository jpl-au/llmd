package host

import (
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestActivitiesRecent seeds the store, then checks the number of
// events returned for limit and, when wantEvent is set, that an event
// with its Type, Author and (if non-empty) Subject is present.
func TestActivitiesRecent(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, c sdk.Context)
		limit     int
		wantLen   int
		wantEvent sdk.Activity // zero value skips the check
	}{
		{
			name:    "empty store",
			limit:   10,
			wantLen: 0,
		},
		{
			name: "document write",
			setup: func(t *testing.T, c sdk.Context) {
				if err := c.Documents.Write("doc", []byte("hello"), sdk.WriteOpts{Author: "alice", Message: "first"}); err != nil {
					t.Fatalf("Write(doc) error = %v", err)
				}
			},
			limit:     10,
			wantLen:   1,
			wantEvent: sdk.Activity{Type: "document", Subject: "doc", Author: "alice"},
		},
		{
			name: "task add",
			setup: func(t *testing.T, c sdk.Context) {
				addTask(t, c, "test task", nil, sdk.TaskAddOpts{Author: "bob"})
			},
			limit:     10,
			wantLen:   1,
			wantEvent: sdk.Activity{Type: "task", Author: "bob"},
		},
		{
			name: "limit",
			setup: func(t *testing.T, c sdk.Context) {
				writeDoc(t, c, "a", "x")
				writeDoc(t, c, "b", "x")
				writeDoc(t, c, "c", "x")
			},
			limit:   2,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			if tt.setup != nil {
				tt.setup(t, c)
			}
			events, err := c.Activities.Recent(tt.limit)
			if err != nil {
				t.Fatalf("Recent(%d) error = %v", tt.limit, err)
			}
			if len(events) != tt.wantLen {
				t.Errorf("Recent(%d) returned %d events, want %d", tt.limit, len(events), tt.wantLen)
			}
			if tt.wantEvent == (sdk.Activity{}) {
				return
			}
			found := false
			for _, e := range events {
				if e.Type == tt.wantEvent.Type && e.Author == tt.wantEvent.Author &&
					(tt.wantEvent.Subject == "" || e.Subject == tt.wantEvent.Subject) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Recent(%d) = %+v, want event matching %+v", tt.limit, events, tt.wantEvent)
			}
		})
	}
}
