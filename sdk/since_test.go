package sdk

import (
	"testing"
	"time"
)

// TestParseSince covers the accepted inputs (empty, duration shorthand
// including whole days, RFC 3339) and rejection of anything else. Duration cases
// are relative to time.Now(), so they are checked against wantAgo
// with a one-minute tolerance rather than an exact instant.
func TestParseSince(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time     // exact instant (RFC 3339 and empty cases)
		wantAgo time.Duration // approximate offset from now (duration cases)
		wantErr bool
	}{
		{name: "empty is zero time", input: ""},
		{name: "duration minutes", input: "5m", wantAgo: 5 * time.Minute},
		{name: "duration hours", input: "1h", wantAgo: time.Hour},
		{name: "duration compound", input: "2h30m", wantAgo: 2*time.Hour + 30*time.Minute},
		{name: "days", input: "7d", wantAgo: 7 * 24 * time.Hour},
		{name: "rfc3339", input: "2026-01-15T10:00:00Z", want: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)},
		{name: "invalid word", input: "yesterday", wantErr: true},
		{name: "invalid date only", input: "2026-01-15", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			got, err := ParseSince(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSince(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.wantAgo != 0 {
				diff := before.Sub(got)
				if diff < tt.wantAgo-time.Minute || diff > tt.wantAgo+time.Minute {
					t.Errorf("ParseSince(%q) = %v ago, want ~%v", tt.input, diff, tt.wantAgo)
				}
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("ParseSince(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
