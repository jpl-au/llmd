// Package duration provides parsing for human-readable duration strings.
//
// Users specify durations as "7d" (days), "4w" (weeks), or "3m" (months) rather
// than Go's time.Duration format. This matches common CLI conventions and is
// more intuitive for vacuum --older-than and similar retention policies.
package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Parse parses duration strings in the format: Nd (days), Nw (weeks), Nm (months).
// Examples: "7d" = 7 days, "4w" = 4 weeks, "3m" = 3 months (30 days).
func Parse(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([dwm])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s (use 7d, 4w, or 3m)", s)
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		// Regex ensures digits only, but handle error for correctness
		return 0, fmt.Errorf("invalid number: %w", err)
	}

	switch matches[2] {
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "w":
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %s", matches[2])
	}
}
