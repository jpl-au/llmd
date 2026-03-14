package search

import (
	"context"
	"fmt"
	"strings"
)

// snippets implements ModeSnippets using FTS5's native snippet() function.
// Returns highlighted excerpts around matching terms. Each snippet includes:
//   - <b></b> markers around matched terms
//   - "..." ellipsis where content is omitted
//   - Up to 64 tokens of context
//
// This is the most efficient mode for generating search result previews
// as FTS5 handles the snippet generation in SQL.
func (s *Search) snippets(ctx context.Context, query string, opt Options) ([]Result, error) {
	var b strings.Builder
	var args []any

	// FTS5 snippet() parameters:
	//   - fts_table: the FTS table name
	//   - column_index: 2 = content column (path=0, key=1, content=2)
	//   - start_marker: text before matched terms
	//   - end_marker: text after matched terms
	//   - ellipsis: text for omitted content
	//   - max_tokens: maximum tokens in snippet
	b.WriteString(`
		SELECT c.key, c.path, snippet(content_fts, 2, '<b>', '</b>', '...', 64)
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
		var key, path, snippet string
		if err := rows.Scan(&key, &path, &snippet); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, Result{
			Path: path,
			Key:  key,
			Matches: []Match{{
				Text: snippet,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}
