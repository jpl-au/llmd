package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
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

// parseMarkdown parses content using goldmark and extracts sections.
// Each heading (h1-h6) starts a new section that continues until the
// next heading or end of document. If the document has no headings,
// returns a single section containing the entire content.
func parseMarkdown(content string) []section {
	source := []byte(content)
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))

	var sections []section
	var headings []struct {
		title string
		start int
	}

	lines := strings.Split(content, "\n")
	lineOffsets := make([]int, len(lines)+1)
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1
	}
	lineOffsets[len(lines)] = len(content)

	// Collect all headings
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			var title strings.Builder
			for c := h.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					title.Write(t.Segment.Value(source))
				}
			}
			line := lineFromOffset(lineOffsets, int(h.Lines().At(0).Start))
			headings = append(headings, struct {
				title string
				start int
			}{title.String(), line})
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return nil
	}

	if len(headings) == 0 {
		// No headings: treat entire doc as one section
		return []section{{
			header:    "",
			startLine: 1,
			endLine:   len(lines),
			text:      content,
		}}
	}

	// Build sections from headings
	for i, h := range headings {
		endLine := len(lines)
		if i+1 < len(headings) {
			endLine = headings[i+1].start - 1
		}

		startIdx := 0
		endIdx := len(content)
		if h.start > 1 && h.start-1 < len(lineOffsets) {
			startIdx = lineOffsets[h.start-1]
		}
		if endLine < len(lineOffsets) {
			endIdx = lineOffsets[endLine]
		}

		sections = append(sections, section{
			header:    h.title,
			startLine: h.start,
			endLine:   endLine,
			text:      content[startIdx:endIdx],
		})
	}

	// If there's content before the first heading, add it as a section
	if headings[0].start > 1 {
		endIdx := lineOffsets[headings[0].start-1]
		preamble := section{
			header:    "",
			startLine: 1,
			endLine:   headings[0].start - 1,
			text:      content[:endIdx],
		}
		sections = append([]section{preamble}, sections...)
	}

	return sections
}

// lineFromOffset converts a byte offset to a 1-indexed line number.
// The offsets slice maps line indices to their starting byte positions.
func lineFromOffset(offsets []int, offset int) int {
	for i, o := range offsets {
		if o > offset {
			return i
		}
	}
	return len(offsets)
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
