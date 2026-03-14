package search

import (
	"context"
	"fmt"
	"strings"
)

// FullText searches documents using SQLite FTS5.
//
// The query parameter accepts FTS5 query syntax:
//   - Simple terms: "hello world" (implicit AND)
//   - Phrases: `"exact phrase"`
//   - Prefix matching: "auto*"
//   - Boolean operators: "foo AND bar", "foo OR bar", "NOT foo"
//   - Grouping: "(foo OR bar) AND baz"
//
// Results are ordered by FTS5 rank (relevance score).
//
// Returns [ErrInvalidQuery] if the query syntax is malformed.
//
// Example:
//
//	// Find documents containing "error" in docs/, return matching lines
//	results, err := s.FullText(ctx, "error", Options{
//	    Path:    "docs/",
//	    Mode:    ModeLines,
//	    Context: 2,
//	})
func (s *Search) FullText(ctx context.Context, query string, opts ...Options) ([]Result, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	switch opt.Mode {
	case ModeSnippets:
		return s.snippets(ctx, query, opt)
	case ModePaths:
		return s.paths(ctx, query, opt)
	case ModeLines:
		return s.lines(ctx, query, opt)
	case ModeSections:
		return s.sections(ctx, query, opt)
	default:
		return s.full(ctx, query, opt)
	}
}

// full returns entire document content (default mode).
func (s *Search) full(ctx context.Context, query string, opt Options) ([]Result, error) {
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
		results = append(results, Result{
			Path: path,
			Key:  key,
			Matches: []Match{{
				Line: 1,
				Text: content,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}

// paths returns only document paths (no content).
func (s *Search) paths(ctx context.Context, query string, opt Options) ([]Result, error) {
	var b strings.Builder
	var args []any

	b.WriteString(`
		SELECT c.key, c.path
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
		var key, path string
		if err := rows.Scan(&key, &path); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, Result{
			Path: path,
			Key:  key,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}
