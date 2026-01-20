// Package key provides unique identifier generation for documents and entities.
package key

import (
	"errors"
	"strconv"
	"sync/atomic"
	"time"
)

const padding = "000000000"

var (
	ErrInvalidLength = errors.New("key must be 9 characters")
	ErrInvalidChar   = errors.New("key must be lowercase base36")
)

// counter ensures uniqueness within the same millisecond.
var counter atomic.Int64

// Generate returns a new unique 9-character base36 key.
// Uses timestamp in milliseconds plus an atomic counter for uniqueness.
func Generate() string {
	// Combine timestamp with counter to ensure uniqueness
	// Counter always increases, ensuring no duplicates even in tight loops
	ms := time.Now().UnixMilli()
	c := counter.Add(1)
	return GenerateAt(ms + c)
}

// GenerateAt returns a 9-character base36 key from the given millisecond timestamp.
func GenerateAt(ms int64) string {
	s := strconv.FormatInt(ms, 36)
	if len(s) >= 9 {
		return s
	}
	return padding[:9-len(s)] + s
}

// Validate checks that a key is a valid 9-character base36 string.
func Validate(k string) error {
	if len(k) != 9 {
		return ErrInvalidLength
	}
	for _, c := range k {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			return ErrInvalidChar
		}
	}
	return nil
}
