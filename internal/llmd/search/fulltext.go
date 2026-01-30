package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// FullText searches documents using FTS5.
// The FTS index contains the latest non-deleted version of each document.
func (s *Search) FullText(ctx context.Context, query string, opts ...Options) ([]document.Document, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	var b strings.Builder
	var args []any

	b.WriteString(`
		SELECT c.id, c.key, c.namespace, c.path, c.content, c.version, c.hash,
		       c.author, c.message, c.source, c.mime, c.meta, c.created_at
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

	return scan(rows)
}

func scan(rows *sql.Rows) ([]document.Document, error) {
	var results []document.Document

	for rows.Next() {
		var doc document.Document
		var meta, message, mime sql.NullString

		err := rows.Scan(
			&doc.ID, &doc.Key, &doc.Namespace, &doc.Path, &doc.Content,
			&doc.Version, &doc.Hash, &doc.Author, &message, &doc.Source,
			&mime, &meta, &doc.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if message.Valid {
			doc.Message = message.String
		}
		if mime.Valid {
			doc.MIME = mime.String
		}
		if meta.Valid && meta.String != "" {
			var m document.Meta
			if err := json.Unmarshal([]byte(meta.String), &m); err == nil {
				doc.Meta = &m
			}
		}

		results = append(results, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}
