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
//
// Platform-specific handling: The Normalise and Direct functions are implemented
// separately for Windows and Unix systems (see path_windows.go, path_unix.go).
// This ensures correct backslash handling on each platform.
package path

import "errors"

// ErrInvalid indicates the provided document path is invalid.
var ErrInvalid = errors.New("invalid document path")

// ErrTooLong indicates the document path exceeds the configured maximum length.
var ErrTooLong = errors.New("document path too long")
