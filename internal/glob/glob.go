// Package glob provides glob pattern matching for document paths.
//
// Extends filepath.Match with ** support for matching any path segments.
// This enables patterns like "docs/**" to match all documents under docs/,
// regardless of nesting depth.
package glob

import (
	"path/filepath"
	"strings"
)

// Match reports whether path matches the glob pattern.
// Supports standard glob patterns (*, ?) plus ** for matching any path segments.
func Match(pattern, path string) bool {
	// Normalise pattern
	pattern = strings.TrimSuffix(pattern, ".md")
	pattern = filepath.ToSlash(pattern)

	// Handle ** (match any path segments)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}
			if suffix == "" {
				return true
			}
			// Match suffix as a glob pattern against all path segments
			segments := strings.Split(path, "/")
			for i := range segments {
				tail := strings.Join(segments[i:], "/")
				if m, _ := filepath.Match(suffix, tail); m {
					return true
				}
				// Also try matching just the segment itself
				if m, _ := filepath.Match(suffix, segments[i]); m {
					return true
				}
			}
			return false
		}
	}

	// Use filepath.Match for simple patterns
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Try matching just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}
