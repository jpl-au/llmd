// Package hash provides content hashing.
package hash

import (
	"encoding/hex"

	"golang.org/x/crypto/blake2b"
)

// Blake2b returns the blake2b-128 hash of content as a 32-character hex string.
func Blake2b(content string) string {
	h, _ := blake2b.New(16, nil) // 16 bytes = 128 bits
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
