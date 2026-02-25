// Package hash provides content hashing for document change detection
// and deduplication.
//
// Two algorithms are available:
//   - [XXH3]: fast 64-bit hash used for change detection during imports
//     and writes. Speed matters here because every document write
//     computes a hash.
//   - [Blake2b]: cryptographic 128-bit hash used where collision
//     resistance matters more than speed (e.g. content addressing).
package hash

import (
	"fmt"

	"github.com/zeebo/xxh3"
)

// XXH3 returns the xxh3-64 hash of content as a 16-character hex string.
func XXH3(content string) string {
	h := xxh3.HashString(content)
	return fmt.Sprintf("%016x", h)
}
