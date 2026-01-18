// Package path provides document path normalisation and validation utilities.
//
// All document paths in llmd pass through this package before storage or retrieval.
// Validation ensures paths are safe for both database storage and filesystem mirroring.
//
// Security: Path traversal attacks are blocked by rejecting any path containing "..".
// Combined with os.OpenRoot in the sync package, this provides defence-in-depth
// against escaping the document store.
//
// Normalisation rules:
//   - Paths use forward slashes (Windows-compatible)
//   - No leading or trailing slashes
//   - No "." or ".." components
//   - Empty paths are rejected
//   - .md extension is stripped (docs/readme.md becomes docs/readme)
package path

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrInvalid indicates the provided document path is invalid.
var ErrInvalid = errors.New("invalid document path")

// ErrTooLong indicates the document path exceeds the configured maximum length.
var ErrTooLong = errors.New("document path too long")

// Normalise cleans and validates a document path.
// It ensures paths use forward slashes, have no leading/trailing slashes,
// and contain no directory traversal sequences.
func Normalise(p string) (string, error) {
	if p == "" {
		return "", ErrInvalid
	}

	// Clean the path and convert to forward slashes
	p = filepath.Clean(p)
	p = filepath.ToSlash(p)

	// Remove leading/trailing slashes (must be after ToSlash for Windows)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")

	// Strip .md extension (case-insensitive)
	p = strings.TrimSuffix(p, ".md")
	p = strings.TrimSuffix(p, ".MD")

	// Validate
	if p == "" || p == "." || p == ".." {
		return "", ErrInvalid
	}

	if strings.Contains(p, "..") {
		return "", ErrInvalid
	}

	return p, nil
}
