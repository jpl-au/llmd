// write.go implements document creation and modification operations.
//
// Separated from the main store file to isolate mutating operations. All
// writes create new versions rather than updating in place - this enables
// full version history and recovery from accidental changes.
//
// Design: Write operations use transactions to ensure atomicity. The version
// number is computed as MAX(version)+1 within the transaction to prevent
// race conditions in concurrent write scenarios.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/validate"
)

// Write creates a new version of a document, preserving all previous versions.
// New documents start at version 1; existing documents increment from their max.
// The version is computed inside a transaction to prevent race conditions when
// multiple writers target the same path concurrently.
func (s *SQLiteStore) Write(ctx context.Context, path, content string, opts WriteOptions) error {
	path, err := validate.Path(path, opts.MaxPath)
	if err != nil {
		return err
	}
	if err := validate.Content(content, opts.MaxContent); err != nil {
		return err
	}

	return s.Tx(ctx, func(tx *sql.Tx) error {
		var maxVer int
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM documents WHERE path = ?`, path).Scan(&maxVer)
		if err != nil {
			return fmt.Errorf("get max version: %w", err)
		}

		id, err := genID()
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO documents (key, path, content, version, author, message, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, path, content, maxVer+1, opts.Author, opts.Message, time.Now().Unix())
		if err != nil {
			return fmt.Errorf("insert document: %w", err)
		}
		return nil
	})
}

// Delete soft-deletes a document by setting deleted_at timestamp.
// All versions are marked deleted together - you can't delete individual versions.
// Associated links are cascade-deleted to maintain referential integrity.
// Returns ErrNotFound if the document doesn't exist or is already deleted.
func (s *SQLiteStore) Delete(ctx context.Context, path string, opts DeleteOptions) error {
	path, err := validate.Path(path, opts.MaxPath)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `UPDATE documents SET deleted_at = ? WHERE path = ? AND deleted_at IS NULL`,
		now, path)
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Cascade soft-delete to associated links (both directions)
	_, err = s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = ?
		WHERE (from_path = ? OR to_path = ?) AND deleted_at IS NULL
	`, now, path, path)
	if err != nil {
		return fmt.Errorf("deleting links for %s: %w", path, err)
	}
	return nil
}

// DeleteVersion soft-deletes a specific version of a document.
// Only the specified version is marked deleted; other versions remain accessible.
// Returns ErrNotFound if the version doesn't exist.
// Unlike Delete, this does NOT cascade to links since the document still exists.
func (s *SQLiteStore) DeleteVersion(ctx context.Context, path string, version int, opts DeleteVersionOptions) error {
	path, err := validate.Path(path, opts.MaxPath)
	if err != nil {
		return err
	}
	if version < 1 {
		return fmt.Errorf("version must be >= 1, got %d", version)
	}

	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `UPDATE documents SET deleted_at = ? WHERE path = ? AND version = ? AND deleted_at IS NULL`,
		now, path, version)
	if err != nil {
		return fmt.Errorf("delete version %d of %s: %w", version, path, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete version %d of %s: %w", version, path, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Restore un-deletes a soft-deleted document by clearing deleted_at.
// All versions are restored together, and associated links are cascade-restored.
// This is the recovery mechanism that makes soft-delete safe - mistakes can be
// undone until vacuum permanently removes the data.
func (s *SQLiteStore) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	path, err := validate.Path(path, opts.MaxPath)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE documents SET deleted_at = NULL WHERE path = ? AND deleted_at IS NOT NULL`, path)
	if err != nil {
		return fmt.Errorf("restore %s: %w", path, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("restore %s: %w", path, err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Cascade restore to associated links (both directions)
	_, err = s.db.ExecContext(ctx, `
		UPDATE links SET deleted_at = NULL
		WHERE (from_path = ? OR to_path = ?) AND deleted_at IS NOT NULL
	`, path, path)
	if err != nil {
		return fmt.Errorf("restoring links for %s: %w", path, err)
	}
	return nil
}
