// links.go implements document relationship management for knowledge graphs.
//
// Separated from tags.go because links represent relationships between
// documents (edges in a graph), while tags are labels on individual documents
// (node properties). Links have two endpoints and can be traversed in either
// direction.
//
// Design: Links are soft-deleted independently of documents. When a document
// is deleted, its links are also soft-deleted to maintain referential integrity.
// Links can be restored if both endpoint documents still exist.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/validate"
)

// Link creates a relationship between documents. Avoids generating unused IDs
// by checking for existing links first.
func (s *SQLiteStore) Link(ctx context.Context, from, to, tag string, opts LinkOptions) (string, error) {
	if _, err := validate.Path(from, opts.MaxPath); err != nil {
		return "", err
	}
	if _, err := validate.Path(to, opts.MaxPath); err != nil {
		return "", err
	}
	if err := validate.Link(from, to); err != nil {
		return "", err
	}

	// First try to restore a soft-deleted link (avoids generating unused IDs)
	result, err := s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = NULL
		WHERE from_path = ? AND from_source = ? AND to_path = ? AND to_source = ? AND tag = ? AND deleted_at IS NOT NULL
	`, from, opts.FromSource, to, opts.ToSource, tag)
	if err != nil {
		return "", fmt.Errorf("restoring link: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		// Restored existing link, return its ID
		var id string
		err := s.db.QueryRowContext(ctx, `
			SELECT id FROM links
			WHERE from_path = ? AND from_source = ? AND to_path = ? AND to_source = ? AND tag = ? AND deleted_at IS NULL
		`, from, opts.FromSource, to, opts.ToSource, tag).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("fetching restored link ID: %w", err)
		}
		return id, nil
	}

	// Check if active link already exists
	var existingID string
	err = s.db.QueryRowContext(ctx, `
		SELECT id FROM links
		WHERE from_path = ? AND from_source = ? AND to_path = ? AND to_source = ? AND tag = ? AND deleted_at IS NULL
	`, from, opts.FromSource, to, opts.ToSource, tag).Scan(&existingID)
	if err == nil {
		return existingID, nil // Link already exists
	}

	// Insert new link
	now := time.Now().Unix()
	id, err := genID()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO links (id, from_path, from_source, to_path, to_source, tag, created_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`, id, from, opts.FromSource, to, opts.ToSource, tag, now)
	if err != nil {
		return "", fmt.Errorf("creating link: %w", err)
	}
	return id, nil
}

// UnlinkByID soft-deletes a specific link, enabling targeted removal while
// preserving the link for potential recovery until vacuum.
func (s *SQLiteStore) UnlinkByID(ctx context.Context, id string) error {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, now, id)
	if err != nil {
		return fmt.Errorf("unlink %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("unlink %s: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UnlinkByTag soft-deletes all links with a tag, enabling bulk cleanup
// of relationship types (e.g., removing all "depends-on" links at once).
func (s *SQLiteStore) UnlinkByTag(ctx context.Context, tag string, opts LinkOptions) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = ?
		WHERE tag = ? AND from_source = ? AND to_source = ? AND deleted_at IS NULL
	`, now, tag, opts.FromSource, opts.ToSource)
	if err != nil {
		return 0, fmt.Errorf("unlink by tag %s: %w", tag, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("unlink by tag %q: %w", tag, err)
	}
	return n, nil
}

// ListLinks finds connections for a document in either direction,
// enabling relationship discovery regardless of link direction.
func (s *SQLiteStore) ListLinks(ctx context.Context, path, tag string, opts LinkOptions) ([]Link, error) {
	query := `SELECT id, from_path, from_source, to_path, to_source, tag, created_at FROM links
		WHERE ((from_path = ? AND from_source = ?) OR (to_path = ? AND to_source = ?)) AND deleted_at IS NULL`
	args := []any{path, opts.FromSource, path, opts.ToSource}

	if tag != "" {
		query += ` AND tag = ?`
		args = append(args, tag)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list links for %s: %w", path, err)
	}
	defer rows.Close()

	return scanLinks(rows)
}

// ListLinksByTag finds all links with a specific tag for relationship analysis
// and bulk operations on link categories.
func (s *SQLiteStore) ListLinksByTag(ctx context.Context, tag string, opts LinkOptions) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, from_path, from_source, to_path, to_source, tag, created_at FROM links
		WHERE tag = ? AND from_source = ? AND to_source = ? AND deleted_at IS NULL
		ORDER BY from_path, to_path
	`, tag, opts.FromSource, opts.ToSource)
	if err != nil {
		return nil, fmt.Errorf("list links by tag %s: %w", tag, err)
	}
	defer rows.Close()

	return scanLinks(rows)
}

// ListOrphanLinkPaths identifies documents with no links, helping users
// find isolated content that may need linking or removal. Uses opts.FromSource
// to determine which source table to check for orphans.
func (s *SQLiteStore) ListOrphanLinkPaths(ctx context.Context, opts LinkOptions) ([]string, error) {
	src := opts.FromSource
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT path FROM documents d
		WHERE d.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM links l
			WHERE ((l.from_path = d.path AND l.from_source = ?) OR (l.to_path = d.path AND l.to_source = ?))
			AND l.deleted_at IS NULL
		)
		ORDER BY path
	`, src, src)
	if err != nil {
		return nil, fmt.Errorf("list orphan paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan orphan path: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// DeleteLinksForPath soft-deletes all links for a document,
// maintaining referential integrity when documents are removed.
func (s *SQLiteStore) DeleteLinksForPath(ctx context.Context, path string, opts LinkOptions) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = ?
		WHERE ((from_path = ? AND from_source = ?) OR (to_path = ? AND to_source = ?)) AND deleted_at IS NULL
	`, now, path, opts.FromSource, path, opts.ToSource)
	return err
}

// scanLinks iterates over query results, collecting links into a slice.
func scanLinks(rows *sql.Rows) ([]Link, error) {
	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.FromPath, &l.FromSource, &l.ToPath, &l.ToSource, &l.Tag, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
