package search

import (
	"context"
	"fmt"
	"strings"
)

// sections implements ModeSections: returns markdown sections containing matches.
// Parses each matching document as markdown, identifies section boundaries
// (delimited by headings of any level), and returns only sections that
// contain at least one search term. Content before the first heading is
// treated as its own section with an empty header.
func (s *Search) sections(ctx context.Context, query string, opt Options) ([]Result, error) {
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

	rows, err := s.db.Query(b.String(), args...).WithContext(ctx).Read()
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

		matches := extractSections(content, query)
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

// section represents a markdown section delimited by headings.
type section struct {
	header    string // heading text (empty for preamble before first heading)
	startLine int    // 1-indexed line where section starts
	endLine   int    // 1-indexed line where section ends (inclusive)
	text      string // full section content including heading
}

// extractSections parses content as markdown and returns Match objects
// for each section containing at least one query term.
func extractSections(content, query string) []Match {
	sections := parseMarkdown(content)
	terms := parseTerms(query)

	var matches []Match
	for _, sec := range sections {
		if containsTerms(sec.text, terms) {
			matches = append(matches, Match{
				Line:    sec.startLine,
				Text:    sec.text,
				Section: sec.header,
			})
		}
	}

	return matches
}

// parseMarkdown splits content into sections at markdown heading
// boundaries. Each line starting with one or more '#' characters
// followed by a space begins a new section. Content before the first
// heading is returned as a preamble section with an empty header.
func parseMarkdown(content string) []section {
	lines := strings.Split(content, "\n")

	type heading struct {
		title string
		line  int // 1-indexed
	}

	var headings []heading
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Strip leading '#' characters and require a space after them
		// to avoid matching lines like "#hashtag".
		hashes := 0
		for _, c := range trimmed {
			if c == '#' {
				hashes++
			} else {
				break
			}
		}
		rest := trimmed[hashes:]
		if len(rest) == 0 || rest[0] != ' ' {
			continue
		}
		headings = append(headings, heading{
			title: strings.TrimSpace(rest),
			line:  i + 1,
		})
	}

	if len(headings) == 0 {
		return []section{{
			header:    "",
			startLine: 1,
			endLine:   len(lines),
			text:      content,
		}}
	}

	var sections []section

	// Preamble before the first heading.
	if headings[0].line > 1 {
		end := headings[0].line - 1
		sections = append(sections, section{
			header:    "",
			startLine: 1,
			endLine:   end,
			text:      joinLines(lines, 0, end),
		})
	}

	// Build sections from headings.
	for i, h := range headings {
		endLine := len(lines)
		if i+1 < len(headings) {
			endLine = headings[i+1].line - 1
		}
		sections = append(sections, section{
			header:    h.title,
			startLine: h.line,
			endLine:   endLine,
			text:      joinLines(lines, h.line-1, endLine),
		})
	}

	return sections
}

// joinLines rejoins a slice of lines from start (0-indexed, inclusive)
// to end (1-indexed line number, inclusive) back into a string.
func joinLines(lines []string, start, end int) string {
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

// containsTerms returns true if text contains at least one of the terms.
// Matching is case-insensitive. Returns false for empty terms slice.
func containsTerms(text string, terms []string) bool {
	lower := strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
