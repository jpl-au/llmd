// tags.go implements document tagging operations for organisation and filtering.
//
// Separated from write.go because tags are metadata about documents, not
// document content. Tags have their own lifecycle (can be added/removed
// independently of document versions) and their own soft-delete semantics.
//
// Design: Tags are stored in a separate table with path references rather
// than document IDs. This means tags persist across document versions -
// tagging "docs/readme" applies to all versions, not just the current one.

package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jpl-au/llmd/internal/validate"
)

// ListByTag returns the latest version of documents matching the given prefix and tag.
func (s *SQLiteStore) ListByTag(ctx context.Context, prefix, tag string, includeDeleted, deletedOnly bool, opts TagOptions) ([]Document, error) {
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
		) latest ON d.path = latest.path AND d.version = latest.max_version
		INNER JOIN tags t ON d.path = t.path AND t.source = ? AND t.tag = ? AND t.deleted_at IS NULL`)

	args = append(args, opts.Source, tag)

	switch {
	case deletedOnly:
		b.WriteString(` WHERE d.deleted_at IS NOT NULL`)
	case !includeDeleted:
		b.WriteString(` WHERE d.deleted_at IS NULL`)
	}

	b.WriteString(` ORDER BY d.path`)

	rows, err := s.db.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list by tag %s: %w", tag, err)
	}
	defer rows.Close()

	return s.scanDocuments(rows)
}

// Tag associates a label with a document. Uses upsert to restore soft-deleted
// tags, preventing duplicate entries while allowing re-tagging.
func (s *SQLiteStore) Tag(ctx context.Context, path, tag string, opts TagOptions) error {
	if _, err := validate.Path(path, opts.MaxPath); err != nil {
		return err
	}
	if err := validate.Tag(tag); err != nil {
		return err
	}

	// First try to restore a soft-deleted tag (avoids generating unused IDs)
	result, err := s.db.ExecContext(ctx, `
		UPDATE tags SET deleted_at = NULL
		WHERE path = ? AND source = ? AND tag = ? AND deleted_at IS NOT NULL
	`, path, opts.Source, tag)
	if err != nil {
		return fmt.Errorf("restoring tag: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		return nil // Restored existing tag
	}

	// Check if tag already exists (active)
	var exists int
	err = s.db.QueryRowContext(ctx, `
		SELECT 1 FROM tags WHERE path = ? AND source = ? AND tag = ? AND deleted_at IS NULL
	`, path, opts.Source, tag).Scan(&exists)
	if err == nil {
		return nil // Tag already exists
	}

	// Insert new tag
	now := time.Now().Unix()
	id, err := genID()
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tags (id, path, source, tag, created_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, NULL)
	`, id, path, opts.Source, tag, now)
	if err != nil {
		return fmt.Errorf("adding tag: %w", err)
	}
	return nil
}

// Untag soft-deletes a tag, preserving it for potential recovery until vacuum.
func (s *SQLiteStore) Untag(ctx context.Context, path, tag string, opts TagOptions) error {
	if _, err := validate.Path(path, opts.MaxPath); err != nil {
		return err
	}
	if err := validate.Tag(tag); err != nil {
		return err
	}

	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `UPDATE tags SET deleted_at = ? WHERE path = ? AND source = ? AND tag = ? AND deleted_at IS NULL`,
		now, path, opts.Source, tag)
	if err != nil {
		return fmt.Errorf("untag %s from %s: %w", tag, path, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("untag %s from %s: %w", tag, path, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTags returns all unique tags, optionally filtered by document path.
// When path is empty, returns all tags in the system for discovery and autocomplete.
// When path is provided, returns only tags on that specific document.
func (s *SQLiteStore) ListTags(ctx context.Context, path string, opts TagOptions) ([]string, error) {
	query := `SELECT DISTINCT tag FROM tags WHERE deleted_at IS NULL AND source = ?`
	args := []any{opts.Source}

	if path != "" {
		query += ` AND path = ?`
		args = append(args, path)
	}
	query += ` ORDER BY tag`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// PathsWithTag returns all document paths with a specific tag.
// Enables tag-based navigation - "show me all documents tagged 'urgent'".
func (s *SQLiteStore) PathsWithTag(ctx context.Context, tag string, opts TagOptions) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM tags WHERE tag = ? AND source = ? AND deleted_at IS NULL ORDER BY path`, tag, opts.Source)
	if err != nil {
		return nil, fmt.Errorf("list paths with tag %s: %w", tag, err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan path: %w", err)
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
