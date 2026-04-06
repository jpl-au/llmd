package hash

import (
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/blake2b"
)

// Blake2b returns the blake2b-128 hash of content as a 32-character hex string.
func Blake2b(content string) string {
	// blake2b.New cannot fail with a nil key and valid size.
	h, err := blake2b.New(16, nil)
	if err != nil {
		panic(fmt.Sprintf("blake2b.New: %v", err))
	}
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
