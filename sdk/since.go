package sdk

import (
	"fmt"
	"time"
)

// ParseSince parses a --since value into a time.Time. It accepts two
// formats:
//
//   - Duration shorthand: "5m", "1h", "30s", "2h30m" - subtracted
//     from time.Now().
//   - RFC 3339 timestamp: "2026-03-16T04:00:00Z" - used as-is.
//
// Returns the zero time and an error if neither format matches.
func ParseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q: use a duration (5m, 1h) or RFC 3339 timestamp", s)
}
