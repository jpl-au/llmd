// Package hash provides content hashing for document change detection
// and deduplication. XXH3 is a fast 64-bit hash; speed matters because
// every document write computes one.
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
