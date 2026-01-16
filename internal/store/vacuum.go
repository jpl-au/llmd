// vacuum.go implements permanent deletion of soft-deleted data.
//
// Separated because vacuum is a destructive, irreversible operation with
// different semantics than soft-delete. Vacuum should be called deliberately,
// not as part of normal operations.
//
// Design: Soft-delete enables recovery; vacuum removes that safety net.
// The olderThan parameter allows keeping recent deletions recoverable while
// cleaning up old trash. This balances storage efficiency against the
// "oops I deleted that last week" recovery scenario.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Vacuum permanently removes soft-deleted data from the database.
// Parameters:
//   - olderThan: if non-nil, only delete items deleted before this duration ago
//   - path: if non-empty, only delete items matching this path prefix
//
// Returns the total number of rows deleted across all tables.
func (s *SQLiteStore) Vacuum(ctx context.Context, olderThan *time.Duration, path string) (int64, error) {
	var totalDeleted int64

	err := s.Tx(ctx, func(tx *sql.Tx) error {
		// Build cutoff condition
		var cutoff int64
		if olderThan != nil {
			cutoff = time.Now().Add(-*olderThan).Unix()
		}

		// Delete soft-deleted documents
		docQuery := `DELETE FROM documents WHERE deleted_at IS NOT NULL`
		var docArgs []any
		if olderThan != nil {
			docQuery += ` AND deleted_at < ?`
			docArgs = append(docArgs, cutoff)
		}
		if path != "" {
			docQuery += ` AND path LIKE ?`
			docArgs = append(docArgs, path+"%")
		}

		result, err := tx.ExecContext(ctx, docQuery, docArgs...)
		if err != nil {
			return fmt.Errorf("vacuum documents: %w", err)
		}
		if n, err := result.RowsAffected(); err == nil {
			totalDeleted += n
		}

		// Delete soft-deleted tags
		tagQuery := `DELETE FROM tags WHERE deleted_at IS NOT NULL`
		var tagArgs []any
		if olderThan != nil {
			tagQuery += ` AND deleted_at < ?`
			tagArgs = append(tagArgs, cutoff)
		}
		if path != "" {
			tagQuery += ` AND path LIKE ?`
			tagArgs = append(tagArgs, path+"%")
		}
		result, err = tx.ExecContext(ctx, tagQuery, tagArgs...)
		if err != nil {
			return fmt.Errorf("vacuum tags: %w", err)
		}
		if n, err := result.RowsAffected(); err == nil {
			totalDeleted += n
		}

		// Delete soft-deleted links
		linkQuery := `DELETE FROM links WHERE deleted_at IS NOT NULL`
		var linkArgs []any
		if olderThan != nil {
			linkQuery += ` AND deleted_at < ?`
			linkArgs = append(linkArgs, cutoff)
		}
		if path != "" {
			linkQuery += ` AND (from_path LIKE ? OR to_path LIKE ?)`
			linkArgs = append(linkArgs, path+"%", path+"%")
		}
		result, err = tx.ExecContext(ctx, linkQuery, linkArgs...)
		if err != nil {
			return fmt.Errorf("vacuum links: %w", err)
		}
		if n, err := result.RowsAffected(); err == nil {
			totalDeleted += n
		}

		// Clean up orphaned tags (tags for paths with no remaining documents)
		result, err = tx.ExecContext(ctx, `DELETE FROM tags WHERE path NOT IN (SELECT DISTINCT path FROM documents)`)
		if err != nil {
			return fmt.Errorf("vacuum orphan tags: %w", err)
		}
		if n, err := result.RowsAffected(); err == nil {
			totalDeleted += n
		}

		// Clean up orphaned links (links pointing to non-existent documents)
		result, err = tx.ExecContext(ctx, `DELETE FROM links WHERE
			from_path NOT IN (SELECT DISTINCT path FROM documents) OR
			to_path NOT IN (SELECT DISTINCT path FROM documents)`)
		if err != nil {
			return fmt.Errorf("vacuum orphan links: %w", err)
		}
		if n, err := result.RowsAffected(); err == nil {
			totalDeleted += n
		}

		return nil
	})

	if err != nil {
		return 0, err
	}
	return totalDeleted, nil
}
