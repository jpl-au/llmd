// read.go implements document retrieval operations for the SQLite store.
//
// Separated from the main store file to isolate read-only query logic. These
// operations never modify data, enabling clearer reasoning about side effects
// and potential future read replica support.
//
// Design: All read operations work with the "latest version" concept - when
// multiple versions exist, we return the highest version number unless a
// specific version is requested. The includeDeleted flag controls whether
// soft-deleted documents are visible.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Latest returns the highest version of a document at the given path.
// The includeDeleted flag enables reading soft-deleted documents for recovery
// workflows - without it, deleted documents are invisible to prevent accidental use.
func (s *SQLiteStore) Latest(ctx context.Context, path string, includeDeleted bool) (*Document, error) {
	query := `SELECT id, key, path, content, version, author, message, created_at, deleted_at
		FROM documents WHERE path = ?`
	if !includeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	query += ` ORDER BY version DESC LIMIT 1`

	return s.scanDocument(s.db.QueryRowContext(ctx, query, path))
}

// Version returns a specific historical version of a document.
// This enables audit, comparison, and rollback operations. Unlike Latest,
// version queries don't filter by deleted_at because you may need to examine
// the exact state at a point in time regardless of current deletion status.
func (s *SQLiteStore) Version(ctx context.Context, path string, version int) (*Document, error) {
	query := `SELECT id, key, path, content, version, author, message, created_at, deleted_at
		FROM documents WHERE path = ? AND version = ?`
	return s.scanDocument(s.db.QueryRowContext(ctx, query, path, version))
}

// ByKey retrieves a document by its 8-character unique key.
// Keys provide stable external references that survive renames - useful for
// URLs, cross-references, and integrations that need permanent document IDs.
func (s *SQLiteStore) ByKey(ctx context.Context, key string) (*Document, error) {
	query := `SELECT id, key, path, content, version, author, message, created_at, deleted_at
		FROM documents WHERE key = ?`
	return s.scanDocument(s.db.QueryRowContext(ctx, query, key))
}

// List returns the latest version of all documents matching a path prefix.
// The subquery finds max versions per path first, then joins to get full documents.
// This two-step approach is more efficient than alternatives for SQLite.
func (s *SQLiteStore) List(ctx context.Context, prefix string, includeDeleted bool, deletedOnly bool) ([]Document, error) {
	var b strings.Builder
	b.WriteString(`SELECT d.id, d.key, d.path, d.content, d.version, d.author, d.message, d.created_at, d.deleted_at
		FROM documents d
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
		) latest ON d.path = latest.path AND d.version = latest.max_version`)

	switch {
	case deletedOnly:
		b.WriteString(` WHERE d.deleted_at IS NOT NULL`)
	case !includeDeleted:
		b.WriteString(` WHERE d.deleted_at IS NULL`)
	}

	b.WriteString(` ORDER BY d.path`)

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}

// ListPaths returns only document paths without content.
// This enables efficient glob matching and tree displays without loading
// potentially large document content into memory.
func (s *SQLiteStore) ListPaths(ctx context.Context, prefix string) ([]string, error) {
	q := `SELECT DISTINCT path FROM documents WHERE deleted_at IS NULL`
	var args []any

	if prefix != "" {
		q += ` AND path LIKE ?`
		args = append(args, prefix+"%")
	}
	q += ` ORDER BY path`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan path: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// History returns all versions of a document in descending order (newest first).
// The limit parameter prevents unbounded queries on documents with many versions.
// Used for audit trails, version selection UIs, and rollback decisions.
func (s *SQLiteStore) History(ctx context.Context, path string, limit int, includeDeleted bool) ([]Document, error) {
	query := `SELECT id, key, path, content, version, author, message, created_at, deleted_at
		FROM documents WHERE path = ?`
	args := []any{path}

	if !includeDeleted {
		query += ` AND deleted_at IS NULL`
	}
	query += ` ORDER BY version DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list history for %s: %w", path, err)
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}

// Count returns the number of distinct active documents matching a prefix.
// Uses COUNT(DISTINCT path) rather than counting rows because each document
// may have multiple versions - we want document count, not version count.
func (s *SQLiteStore) Count(ctx context.Context, prefix string) (int64, error) {
	query := `SELECT COUNT(DISTINCT path) FROM documents WHERE deleted_at IS NULL`
	var args []any

	if prefix != "" {
		query += ` AND path LIKE ?`
		args = append(args, prefix+"%")
	}

	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Meta returns document metadata without content.
// Useful for listings, dashboards, and size checks where loading full content
// would be wasteful. The size is computed via length(content) in SQL to avoid
// transferring content over the connection.
func (s *SQLiteStore) Meta(ctx context.Context, path string) (*DocumentMeta, error) {
	var m DocumentMeta
	var msg sql.NullString

	// Query filters deleted_at IS NULL, so we don't need to scan it
	err := s.db.QueryRowContext(ctx, `
		SELECT key, path, version, author, message, created_at, length(content)
		FROM documents
		WHERE path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, path).Scan(&m.Key, &m.Path, &m.Version, &m.Author, &msg, &m.CreatedAt, &m.Size)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get meta for %s: %w", path, err)
	}

	if msg.Valid {
		m.Message = msg.String
	}
	return &m, nil
}

// Exists checks if an active document exists at the given path.
// Uses SELECT 1 ... LIMIT 1 for efficiency - we only need to know if at least
// one row exists, not count them or fetch data. Used for pre-flight validation
// before operations that require the document to exist.
func (s *SQLiteStore) Exists(ctx context.Context, path string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM documents WHERE path = ? AND deleted_at IS NULL LIMIT 1`, path).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check exists %s: %w", path, err)
	}
	return true, nil
}
