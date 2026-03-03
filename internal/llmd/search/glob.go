package search

import (
	"context"
	"path"
	"strings"
)

// Glob searches for documents matching a glob pattern.
// Supports *, **, and ? wildcards.
func (s *Search) Glob(ctx context.Context, pattern string, opts ...Options) ([]string, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Validate pattern
	if err := validateGlob(pattern); err != nil {
		return nil, err
	}

	// Get all document paths (latest versions only)
	query := `
		SELECT DISTINCT path FROM content
		WHERE namespace = 'core:document' AND deleted_at IS NULL
		AND version = (
			SELECT MAX(version) FROM content c2
			WHERE c2.namespace = namespace AND c2.path = path
		)
		ORDER BY path
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}

		if matchGlob(pattern, p) {
			matches = append(matches, p)
			if opt.Limit > 0 && len(matches) >= opt.Limit {
				break
			}
		}
	}

	return matches, rows.Err()
}

// matchGlob matches a doc path against a glob pattern.
// Supports ** for recursive matching.
func matchGlob(pattern, p string) bool {
	// Handle ** (match any number of path segments)
	if strings.Contains(pattern, "**") {
		return matchDoublestar(pattern, p)
	}

	// Use path.Match — store paths always use forward slashes.
	matched, _ := path.Match(pattern, p)
	return matched
}

// validateGlob checks if a glob pattern is valid.
func validateGlob(pattern string) error {
	// Remove ** for validation since path.Match doesn't support it
	test := strings.ReplaceAll(pattern, "**", "*")
	_, err := path.Match(test, "")
	if err != nil {
		return ErrInvalidGlob
	}
	return nil
}

// matchDoublestar handles ** glob patterns.
func matchDoublestar(pattern, p string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// Multiple ** not supported, fall back to simple match
		matched, _ := path.Match(pattern, p)
		return matched
	}

	prefix, suffix := parts[0], parts[1]

	// Check prefix
	if prefix != "" && !strings.HasPrefix(p, prefix) {
		return false
	}

	// Check suffix
	if suffix != "" {
		suffix = strings.TrimPrefix(suffix, "/")
		remaining := strings.TrimPrefix(p, prefix)

		// Try matching suffix at each path segment
		segments := strings.Split(remaining, "/")
		for i := range segments {
			candidate := strings.Join(segments[i:], "/")
			if matched, _ := path.Match(suffix, candidate); matched {
				return true
			}
			// Also try matching just the filename part
			if i == len(segments)-1 {
				if matched, _ := path.Match(suffix, segments[i]); matched {
					return true
				}
			}
		}
		return false
	}

	return true
}
