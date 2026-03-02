// Package path provides document path normalisation and validation utilities.
//
// All document paths in llmd pass through this package before storage or
// retrieval. Validation ensures paths are safe for both database storage
// and filesystem mirroring.
//
// Security: Path traversal attacks are blocked by rejecting any path
// containing "..". Combined with normalisation at the API boundary, this
// provides defence-in-depth against escaping the document store.
//
// Normalisation rules:
//   - Paths use forward slashes (cross-platform)
//   - No leading or trailing slashes
//   - No "." or ".." components
//   - Empty paths are rejected
//   - .md extension is stripped (docs/readme.md becomes docs/readme)
//
// Platform-specific handling: The Normalise and Direct functions are
// implemented separately for Windows and Unix systems (see
// path_windows.go, path_unix.go). This ensures correct backslash
// handling on each platform.
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

// ResolveDB converts a --db shorthand name to a full store path.
// An empty string returns the default path (.llmd/llmd.db). A bare
// name like "docs" becomes .llmd/llmd-docs.db. A path that already
// contains a separator or ends in .db is returned unchanged.
func ResolveDB(name string) string {
	if name == "" {
		return filepath.Join(".llmd", "llmd.db")
	}
	if strings.ContainsAny(name, "/\\") || strings.HasSuffix(name, ".db") {
		return name
	}
	return filepath.Join(".llmd", "llmd-"+name+".db")
}

// ToFS converts a normalised document path to a filesystem path under dir.
// Adds .md extension if the path has no extension.
func ToFS(dir, docPath string) string {
	if filepath.Ext(docPath) == "" {
		docPath += ".md"
	}
	return filepath.Join(dir, filepath.FromSlash(docPath))
}
