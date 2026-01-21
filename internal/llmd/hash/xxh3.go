// Package hash provides content hashing.
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
