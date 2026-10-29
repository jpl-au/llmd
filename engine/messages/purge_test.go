package messages

import (
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"

	"github.com/jpl-au/qwr"
)

// TestPurge covers removing messages by age: which messages a cutoff
// removes, that acknowledgements go with their messages, and that a
// dry run only counts. Three messages are sent at t=1000, 2000 and
// 3000; the first is acknowledged by claude.
func TestPurge(t *testing.T) {
	tests := []struct {
		name     string
		before   int64
		dryRun   bool
		want     int64 // returned count
		wantLeft int   // messages still stored
	}{
		{"cutoff before every message", 500, false, 0, 3},
		{"cutoff removes the oldest", 1_500, false, 1, 2},
		{"cutoff is exclusive", 2_000, false, 1, 2},
		{"cutoff removes two", 2_500, false, 2, 1},
		{"cutoff after every message", 9_000, false, 3, 0},
		{"dry run counts only", 2_500, true, 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			store := New(func() *qwr.Manager { return db }, nil)
			var keys []string
			for i, at := range []int64{1_000, 2_000, 3_000} {
				clocktest.Fixed(t, at)
				m, err := store.Send(t.Context(), SendOptions{Type: "note", Payload: string(rune('a' + i)), Author: "alice", Source: "test"})
				if err != nil {
					t.Fatal(err)
				}
				keys = append(keys, m.Key)
			}
			if err := store.Ack(t.Context(), keys[0], "claude"); err != nil {
				t.Fatal(err)
			}

			got, err := store.Purge(t.Context(), tt.before, tt.dryRun)
			if err != nil {
				t.Fatalf("Purge(%d, %v) error = %v", tt.before, tt.dryRun, err)
			}
			if got != tt.want {
				t.Errorf("Purge(%d, %v) = %d, want %d", tt.before, tt.dryRun, got, tt.want)
			}
			left, err := store.History(t.Context(), "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(left) != tt.wantLeft {
				t.Errorf("after Purge(%d, %v), %d messages remain, want %d", tt.before, tt.dryRun, len(left), tt.wantLeft)
			}
			// Acknowledgements never outlive their message.
			var acks int
			row, err := db.Query("SELECT COUNT(*) FROM message_acks").ReadRow()
			if err != nil {
				t.Fatal(err)
			}
			if err := row.Scan(&acks); err != nil {
				t.Fatal(err)
			}
			wantAcks := 1
			if !tt.dryRun && tt.before > 1_000 {
				wantAcks = 0
			}
			if acks != wantAcks {
				t.Errorf("after Purge(%d, %v), %d acks remain, want %d", tt.before, tt.dryRun, acks, wantAcks)
			}
		})
	}
}
