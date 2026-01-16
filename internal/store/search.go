// search.go implements full-text search using SQLite's FTS5 extension.
//
// Separated from read.go because FTS5 has fundamentally different query
// semantics. Regular reads use exact path matching; FTS5 uses tokenised
// search with its own query syntax (AND, OR, prefix*, phrase "matching").
//
// Design: The FTS5 index includes soft-deleted documents. This enables
// searching trash contents with -D flag. The deletion filter is applied
// at query time via the subquery, not in the index itself.

package store

import (
	"context"
	"strings"
)

// Search performs full-text search using FTS5, returning the latest version
// of each matching document. The query supports FTS5 syntax including AND, OR,
// prefix* matching, and "phrase" queries. Results are filtered by path prefix
// and deletion status according to the flags.
func (s *SQLiteStore) Search(ctx context.Context, query string, prefix string, includeDeleted bool, deletedOnly bool) ([]Document, error) {
	var b strings.Builder
	b.WriteString(`SELECT d.id, d.key, d.path, d.content, d.version, d.author, d.message, d.created_at, d.deleted_at
		FROM documents_fts
		JOIN documents d ON documents_fts.rowid = d.id
		INNER JOIN (
			SELECT path, MAX(version) as max_version FROM documents`)

	var args []any
	var conditions []string

	if prefix != "" {
		conditions = append(conditions, `path LIKE ?`)
		args = append(args, prefix+"%")
	}

	switch {
	case deletedOnly:
		conditions = append(conditions, `deleted_at IS NOT NULL`)
	case !includeDeleted:
		conditions = append(conditions, `deleted_at IS NULL`)
	}

	if len(conditions) > 0 {
		b.WriteString(` WHERE `)
		b.WriteString(strings.Join(conditions, ` AND `))
	}

	b.WriteString(` GROUP BY path
		) latest ON d.path = latest.path AND d.version = latest.max_version
		WHERE documents_fts MATCH ?`)

	args = append(args, query)

	// Note: No need to re-filter by deleted_at here - the subquery already
	// determined the "latest" version considering deletion status, and the
	// join limits results to exactly those versions.

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}
