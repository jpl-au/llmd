// write_move.go implements document rename and copy operations.
//
// Separated from write.go because move/copy have complex transactional
// requirements - they must update document paths AND all references in
// tags and links atomically. A failed move must not leave orphaned references.
//
// Design: Move updates paths in-place rather than creating new versions.
// This preserves version history under the new path. Copy creates a new
// version 1 at the destination, breaking the version chain intentionally.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/validate"
)

// Move renames a document, updating all references in tags and links.
// Returns ErrNotFound if source doesn't exist, ErrAlreadyExists if destination exists.
func (s *SQLiteStore) Move(ctx context.Context, src, dst string, opts MoveOptions) error {
	src, err := validate.Path(src, opts.MaxPath)
	if err != nil {
		return err
	}
	dst, err = validate.Path(dst, opts.MaxPath)
	if err != nil {
		return err
	}

	return s.Tx(ctx, func(tx *sql.Tx) error {
		// Check destination doesn't exist
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ? AND deleted_at IS NULL`, dst).Scan(&n); err != nil {
			return fmt.Errorf("check destination %s: %w", dst, err)
		}
		if n > 0 {
			return ErrAlreadyExists
		}

		res, err := tx.ExecContext(ctx, `UPDATE documents SET path = ? WHERE path = ?`, dst, src)
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", src, dst, err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("move %s to %s: %w", src, dst, err)
		}
		if rows == 0 {
			return ErrNotFound
		}

		// Update tags to point to new path
		if _, err := tx.ExecContext(ctx, `UPDATE tags SET path = ? WHERE path = ?`, dst, src); err != nil {
			return fmt.Errorf("update tags for move %s to %s: %w", src, dst, err)
		}

		// Update links to point to new path (both directions)
		if _, err := tx.ExecContext(ctx, `UPDATE links SET from_path = ? WHERE from_path = ?`, dst, src); err != nil {
			return fmt.Errorf("update link sources for move %s to %s: %w", src, dst, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE links SET to_path = ? WHERE to_path = ?`, dst, src); err != nil {
			return fmt.Errorf("update link targets for move %s to %s: %w", src, dst, err)
		}
		return nil
	})
}

// Copy duplicates a document to a new path, creating version 1 at the destination.
// Returns ErrNotFound if source doesn't exist, ErrAlreadyExists if destination exists.
func (s *SQLiteStore) Copy(ctx context.Context, from, to, copier string, opts CopyOptions) error {
	from, err := validate.Path(from, opts.MaxPath)
	if err != nil {
		return err
	}
	to, err = validate.Path(to, opts.MaxPath)
	if err != nil {
		return err
	}

	return s.Tx(ctx, func(tx *sql.Tx) error {
		// Check destination doesn't exist
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ? AND deleted_at IS NULL`, to).Scan(&n); err != nil {
			return fmt.Errorf("check destination %s: %w", to, err)
		}
		if n > 0 {
			return ErrAlreadyExists
		}

		// Get source document content
		var content string
		err := tx.QueryRowContext(ctx, `
			SELECT content FROM documents
			WHERE path = ? AND deleted_at IS NULL
			ORDER BY version DESC LIMIT 1
		`, from).Scan(&content)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read source %s: %w", from, err)
		}

		// Create copy at version 1, using copier as author to track who performed the copy
		id, err := genID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (key, path, content, version, author, message, created_at)
			VALUES (?, ?, ?, 1, ?, ?, ?)
		`, id, to, content, copier, "Copied from "+from, time.Now().Unix())
		if err != nil {
			return fmt.Errorf("copy %s to %s: %w", from, to, err)
		}
		return nil
	})
}
