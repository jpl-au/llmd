// stats.go implements statistics and metadata queries for operational visibility.
//
// Separated to collect "read-only, aggregate" operations distinct from CRUD.
// These queries power dashboards, admin tools, and vacuum planning without
// modifying data or loading full document content.
//
// Design: All functions avoid loading document content into memory to stay
// efficient on large stores. They use COUNT(), length(), and DISTINCT queries
// to extract metadata directly from the database.

package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// ListDeletedPaths returns paths of soft-deleted documents without loading
// content. Supports trash management UIs and vacuum preview operations.
func (s *SQLiteStore) ListDeletedPaths(ctx context.Context, prefix string) ([]string, error) {
	q := `SELECT DISTINCT path FROM documents WHERE deleted_at IS NOT NULL`
	var args []any

	if prefix != "" {
		q += ` AND path LIKE ?`
		args = append(args, prefix+"%")
	}
	q += ` ORDER BY path`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// ListMeta returns metadata for multiple documents, enabling efficient batch
// queries for dashboards and admin tools that need document info without
// loading full content.
func (s *SQLiteStore) ListMeta(ctx context.Context, prefix string, includeDeleted bool) ([]DocumentMeta, error) {
	q := `SELECT d.key, d.path, d.version, d.author, d.message, d.created_at, d.deleted_at, length(d.content)
		FROM documents d
		INNER JOIN (
			SELECT path, MAX(version) as max_version FROM documents`

	var args []any
	var conditions []string

	if prefix != "" {
		conditions = append(conditions, `path LIKE ?`)
		args = append(args, prefix+"%")
	}

	if !includeDeleted {
		conditions = append(conditions, `deleted_at IS NULL`)
	}

	if len(conditions) > 0 {
		q += ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	q += ` GROUP BY path
		) latest ON d.path = latest.path AND d.version = latest.max_version`

	if !includeDeleted {
		q += ` WHERE d.deleted_at IS NULL`
	}

	q += ` ORDER BY d.path`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []DocumentMeta
	for rows.Next() {
		var m DocumentMeta
		var msg sql.NullString
		if err := rows.Scan(&m.Key, &m.Path, &m.Version, &m.Author, &msg, &m.CreatedAt, &m.DeletedAt, &m.Size); err != nil {
			return nil, err
		}
		if msg.Valid {
			m.Message = msg.String
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// CountDeleted returns the count of soft-deleted documents. Supports vacuum
// preview to show users how many documents will be affected.
func (s *SQLiteStore) CountDeleted(ctx context.Context, prefix string) (int64, error) {
	q := `SELECT COUNT(DISTINCT path) FROM documents WHERE deleted_at IS NOT NULL`
	var args []any

	if prefix != "" {
		q += ` AND path LIKE ?`
		args = append(args, prefix+"%")
	}

	var count int64
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&count)
	return count, err
}

// DeletedBefore returns paths of documents deleted before the given time.
// Enables targeted vacuum operations for cleaning up old trash without
// affecting recently deleted items that users might want to restore.
func (s *SQLiteStore) DeletedBefore(ctx context.Context, t time.Time, prefix string) ([]string, error) {
	cutoff := t.Unix()
	q := `SELECT DISTINCT path FROM documents WHERE deleted_at IS NOT NULL AND deleted_at < ?`
	args := []any{cutoff}

	if prefix != "" {
		q += ` AND path LIKE ?`
		args = append(args, prefix+"%")
	}
	q += ` ORDER BY path`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// VersionCount returns the number of versions for a document without loading
// the full history. Enables version management decisions like pruning old
// versions or displaying version counts in listings.
func (s *SQLiteStore) VersionCount(ctx context.Context, path string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ?`, path).Scan(&count)
	return count, err
}

// ListAuthors returns all distinct authors who have written documents.
// Supports author-based filtering in UIs and audit reporting.
func (s *SQLiteStore) ListAuthors(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT author FROM documents ORDER BY author`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}

// Stats returns aggregate database statistics. Provides operational visibility
// into store utilisation for capacity planning, monitoring dashboards, and
// administrative tooling.
func (s *SQLiteStore) Stats(ctx context.Context) (*Stats, error) {
	var st Stats

	// Active document count (distinct paths, not deleted)
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT path) FROM documents WHERE deleted_at IS NULL`).Scan(&st.Documents)
	if err != nil {
		return nil, err
	}

	// Deleted document count (distinct paths in trash)
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT path) FROM documents WHERE deleted_at IS NOT NULL`).Scan(&st.DeletedDocs)
	if err != nil {
		return nil, err
	}

	// Total version count (all rows in documents table)
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&st.TotalVersions)
	if err != nil {
		return nil, err
	}

	// Active tag count
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags WHERE deleted_at IS NULL`).Scan(&st.Tags)
	if err != nil {
		return nil, err
	}

	// Active link count
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM links WHERE deleted_at IS NULL`).Scan(&st.Links)
	if err != nil {
		return nil, err
	}

	// Distinct author count
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT author) FROM documents`).Scan(&st.Authors)
	if err != nil {
		return nil, err
	}

	// Oldest and newest document timestamps
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MIN(created_at), 0), COALESCE(MAX(created_at), 0) FROM documents`).Scan(&st.OldestDoc, &st.NewestDoc)
	if err != nil {
		return nil, err
	}

	// Oldest deletion timestamp (for vacuum age planning)
	var oldestDeleted sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT MIN(deleted_at) FROM documents WHERE deleted_at IS NOT NULL`).Scan(&oldestDeleted)
	if err != nil {
		return nil, err
	}
	if oldestDeleted.Valid {
		st.OldestDeletedAt = oldestDeleted.Int64
	}

	return &st, nil
}
