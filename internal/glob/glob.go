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
// Returns an error if the pattern is malformed.
func Match(pattern, path string) (bool, error) {
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
				return false, nil
			}
			if suffix == "" {
				return true, nil
			}
			// Match suffix as a glob pattern against all path segments
			segments := strings.Split(path, "/")
			for i := range segments {
				tail := strings.Join(segments[i:], "/")
				m, err := filepath.Match(suffix, tail)
				if err != nil {
					return false, err
				}
				if m {
					return true, nil
				}
				// Also try matching just the segment itself
				m, err = filepath.Match(suffix, segments[i])
				if err != nil {
					return false, err
				}
				if m {
					return true, nil
				}
			}
			return false, nil
		}
	}

	// Use filepath.Match for simple patterns
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false, err
	}
	if matched {
		return true, nil
	}

	// Try matching just the filename
	matched, err = filepath.Match(pattern, filepath.Base(path))
	return matched, err
}
