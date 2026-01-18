//go:build windows

// path_windows.go provides Windows-specific path normalisation.
//
// On Windows, backslashes are native path separators. We use filepath.ToSlash
// which correctly converts them to forward slashes for consistent storage.

package path

import (
	"path/filepath"
	"strings"
)

// Normalise cleans and validates a document path.
// It ensures paths use forward slashes, have no leading/trailing slashes,
// and contain no directory traversal sequences.
func Normalise(p string) (string, error) {
	if p == "" {
		return "", ErrInvalid
	}

	// Clean the path and convert backslashes to forward slashes
	p = filepath.Clean(p)
	p = filepath.ToSlash(p)

	// Remove leading/trailing slashes
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")

	// Strip .md extension (case-insensitive)
	if len(p) > 3 && strings.EqualFold(p[len(p)-3:], ".md") {
		p = p[:len(p)-3]
	}

	// Validate
	if p == "" || p == "." || p == ".." {
		return "", ErrInvalid
	}

	if strings.Contains(p, "..") {
		return "", ErrInvalid
	}

	return p, nil
}

// Direct reports whether path is a direct child of prefix.
// Both paths should use forward slashes. The prefix is normalised
// (backslashes converted, trailing slash removed) to handle raw user input.
//
// Examples (prefix="docs"):
//   - "docs/readme" -> true (direct child)
//   - "docs/api/auth" -> false (nested)
//   - "docs" -> true (exact match)
//
// Examples (prefix=""):
//   - "readme" -> true (top level)
//   - "docs/readme" -> false (nested)
func Direct(path, prefix string) bool {
	// Normalise prefix: convert backslashes and remove trailing slash
	prefix = filepath.ToSlash(prefix)
	prefix = strings.TrimSuffix(prefix, "/")

	// Exact match
	if path == prefix {
		return true
	}

	// Get remainder after prefix
	var remainder string
	if prefix == "" {
		remainder = path
	} else if strings.HasPrefix(path, prefix+"/") {
		remainder = path[len(prefix)+1:]
	} else {
		return false
	}

	// Direct child = no "/" in the remainder
	return !strings.Contains(remainder, "/")
}
