package search

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Match represents a regex match within a document.
type Match struct {
	Path    string   // Document path
	Line    int      // Line number (1-indexed)
	Content string   // Matching line
	Context []string // Surrounding context lines
}

// RegexResult contains regex search results.
type RegexResult struct {
	Matches []Match        // For ModeContent
	Files   []string       // For ModeFiles
	Counts  map[string]int // For ModeCount
}

// Regex searches documents using regular expressions.
// Uses FTS5 to pre-filter candidate documents when possible for better performance.
func (s *Search) Regex(ctx context.Context, pattern string, opts ...Options) (*RegexResult, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	flags := ""
	if opt.IgnoreCase {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, ErrInvalidPattern
	}

	// Try to extract literal terms from the pattern for FTS pre-filtering
	terms := ExtractSearchTerms(pattern)
	ftsQuery := BuildFTSQuery(terms)

	var b strings.Builder
	var args []any

	if ftsQuery != "" {
		// Use FTS5 to find candidate documents
		b.WriteString(`
			SELECT c.path, c.content FROM content c
			JOIN content_fts fts ON fts.rowid = c.id
			WHERE content_fts MATCH ?
		`)
		args = append(args, ftsQuery)

		if opt.Path != "" {
			b.WriteString(" AND c.path LIKE ?")
			args = append(args, opt.Path+"%")
		}

		b.WriteString(" ORDER BY c.path")
	} else {
		// Fall back to scanning all documents
		b.WriteString(`
			SELECT path, content FROM content
			WHERE namespace = 'core:document' AND deleted_at IS NULL
		`)

		if opt.Path != "" {
			b.WriteString(" AND path LIKE ?")
			args = append(args, opt.Path+"%")
		}

		// Latest version per path
		b.WriteString(`
			AND version = (
				SELECT MAX(version) FROM content c2
				WHERE c2.namespace = namespace AND c2.path = path
			)
			ORDER BY path
		`)
	}

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying docs: %w", err)
	}
	defer rows.Close()

	result := &RegexResult{
		Counts: make(map[string]int),
	}

	for rows.Next() {
		var path, content string
		if err := rows.Scan(&path, &content); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		matches := searchContent(re, path, content, opt)

		if opt.InvertMatch {
			if len(matches) == 0 {
				result.Files = append(result.Files, path)
			}
			continue
		}

		if len(matches) > 0 {
			switch opt.Mode {
			case ModeFiles:
				result.Files = append(result.Files, path)
			case ModeCount:
				result.Counts[path] = len(matches)
			case ModeContent:
				result.Matches = append(result.Matches, matches...)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return result, nil
}

func searchContent(re *regexp.Regexp, path, content string, opts Options) []Match {
	lines := strings.Split(content, "\n")
	var matches []Match

	for i, line := range lines {
		if re.MatchString(line) {
			m := Match{
				Path:    path,
				Line:    i + 1,
				Content: line,
			}

			// Add context if requested
			if opts.Context > 0 && opts.Mode == ModeContent {
				start := max(0, i-opts.Context)
				end := min(len(lines), i+opts.Context+1)
				m.Context = lines[start:end]
			}

			matches = append(matches, m)
		}
	}

	return matches
}
