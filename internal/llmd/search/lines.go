package search

import (
	"context"
	"fmt"
	"strings"
)

// lines implements ModeLines: returns individual matching lines with context.
// Each line containing a search term becomes a Match with optional Before/After
// context lines. Lines are deduplicated (a line matching multiple terms appears once).
func (s *Search) lines(ctx context.Context, query string, opt Options) ([]Result, error) {
	var b strings.Builder
	var args []any

	b.WriteString(`
		SELECT c.key, c.path, c.content
		FROM content_fts fts
		JOIN content c ON c.id = fts.rowid
		WHERE content_fts MATCH ?
	`)
	args = append(args, query)

	if opt.Path != "" {
		b.WriteString(" AND c.path LIKE ?")
		args = append(args, opt.Path+"%")
	}

	b.WriteString(" ORDER BY rank")

	if opt.Limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, opt.Limit)
	}

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		if strings.Contains(err.Error(), "fts5") {
			return nil, ErrInvalidQuery
		}
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var key, path, content string
		if err := rows.Scan(&key, &path, &content); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		matches := extractLines(content, query, opt.Context)
		results = append(results, Result{
			Path:    path,
			Key:     key,
			Matches: matches,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}

// extractLines scans content line-by-line for query terms.
// Returns a Match for each line containing at least one term.
// The ctx parameter controls how many context lines to include.
// Matching is case-insensitive. Lines are returned in document order.
func extractLines(content, query string, ctx int) []Match {
	lines := strings.Split(content, "\n")
	terms := parseTerms(query)

	var matches []Match
	seen := make(map[int]bool)

	for lineNum, line := range lines {
		for _, term := range terms {
			col := indexFold(line, term)
			if col >= 0 && !seen[lineNum] {
				seen[lineNum] = true
				matches = append(matches, Match{
					Line:   lineNum + 1, // 1-indexed
					Column: col,
					Text:   line,
					Before: getContext(lines, lineNum, -ctx),
					After:  getContext(lines, lineNum, ctx),
				})
				break
			}
		}
	}

	return matches
}

// parseTerms extracts searchable terms from an FTS5 query string.
// It handles common FTS5 syntax:
//   - Simple words: "hello" -> ["hello"]
//   - Multiple words: "hello world" -> ["hello", "world"]
//   - Quoted phrases: `"hello world"` -> ["hello world"]
//   - Prefix wildcards: "hel*" -> ["hel"] (wildcard stripped)
//   - Negation: "-excluded" -> [] (negated terms dropped)
//
// FTS5 operators (AND, OR, NOT, NEAR) are filtered out.
// Returns lowercase terms for case-insensitive matching.
func parseTerms(query string) []string {
	var terms []string
	var current strings.Builder
	inQuote := false

	for _, r := range query {
		switch {
		case r == '"':
			if inQuote {
				if current.Len() > 0 {
					terms = append(terms, current.String())
					current.Reset()
				}
			}
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				terms = append(terms, strings.TrimSuffix(current.String(), "*"))
				current.Reset()
			}
		case r == '*':
			// Skip wildcard for matching purposes
		default:
			if r != '-' || current.Len() > 0 {
				current.WriteRune(r)
			}
		}
	}

	if current.Len() > 0 {
		terms = append(terms, strings.TrimSuffix(current.String(), "*"))
	}

	// Filter out FTS5 operators
	var filtered []string
	for _, t := range terms {
		t = strings.ToLower(t)
		if t != "and" && t != "or" && t != "not" && t != "near" && t != "" {
			filtered = append(filtered, t)
		}
	}

	return filtered
}

// indexFold returns the byte index of substr in s using case-insensitive
// matching. Returns -1 if substr is not found.
func indexFold(s, substr string) int {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Index(s, substr)
}

// getContext returns surrounding lines from a line array.
// If n < 0, returns |n| lines before lineNum.
// If n > 0, returns n lines after lineNum.
// If n == 0, returns nil.
// Automatically clamps to array bounds.
func getContext(lines []string, lineNum, n int) []string {
	if n == 0 {
		return nil
	}

	var result []string
	if n < 0 {
		// Lines before
		start := lineNum + n
		if start < 0 {
			start = 0
		}
		for i := start; i < lineNum; i++ {
			result = append(result, lines[i])
		}
	} else {
		// Lines after
		end := lineNum + n + 1
		if end > len(lines) {
			end = len(lines)
		}
		for i := lineNum + 1; i < end; i++ {
			result = append(result, lines[i])
		}
	}

	return result
}
