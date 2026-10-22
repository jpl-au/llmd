package sdk

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseSince parses a --since value into a time.Time. It accepts two
// formats:
//
//   - Duration shorthand: "5m", "1h", "30s", "2h30m" - subtracted
//     from time.Now(). Whole days are accepted too: "7d", "30d".
//   - RFC 3339 timestamp: "2026-03-16T04:00:00Z" - used as-is.
//
// Returns the zero time and an error if neither format matches.
func ParseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if days, ok := strings.CutSuffix(s, "d"); ok {
		if n, err := strconv.Atoi(days); err == nil && n >= 0 {
			return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time value %q: use a duration (5m, 1h, 7d) or RFC 3339 timestamp", s)
}
