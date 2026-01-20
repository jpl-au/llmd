package documents

import (
	"context"
	"fmt"
)

// Restore restores a soft-deleted document at the given path.
// All versions of the document are restored.
func (d *Documents) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	result, err := d.db.ExecContext(ctx, `
		UPDATE content SET deleted_at = NULL
		WHERE namespace = ? AND path = ? AND deleted_at IS NOT NULL
	`, namespace, path)
	if err != nil {
		return fmt.Errorf("restoring document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
