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
		setup     func(t *testing.T)
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
			setup: func(t *testing.T) {
				if err := sdk.Documents.Write("doc", []byte("hello"), sdk.WriteOpts{Author: "alice", Message: "first"}); err != nil {
					t.Fatalf("Write(doc) error = %v", err)
				}
			},
			limit:     10,
			wantLen:   1,
			wantEvent: sdk.Activity{Type: "document", Subject: "doc", Author: "alice"},
		},
		{
			name: "task add",
			setup: func(t *testing.T) {
				addTask(t, "test task", nil, sdk.TaskAddOpts{Author: "bob"})
			},
			limit:     10,
			wantLen:   1,
			wantEvent: sdk.Activity{Type: "task", Author: "bob"},
		},
		{
			name: "limit",
			setup: func(t *testing.T) {
				writeDoc(t, "a", "x")
				writeDoc(t, "b", "x")
				writeDoc(t, "c", "x")
			},
			limit:   2,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			if tt.setup != nil {
				tt.setup(t)
			}
			events, err := sdk.Activities.Recent(tt.limit)
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
