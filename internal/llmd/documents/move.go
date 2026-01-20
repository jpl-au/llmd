package documents

import (
	"context"
	"fmt"
)

// Move renames a document from src to dst.
// All versions are moved to the new path.
func (d *Documents) Move(ctx context.Context, src, dst string, opts MoveOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	// Check destination doesn't exist
	if ok, err := d.Exists(ctx, dst); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("destination already exists: %s", dst)
	}

	result, err := d.db.ExecContext(ctx, `
		UPDATE content SET path = ?
		WHERE namespace = ? AND path = ?
	`, dst, namespace, src)
	if err != nil {
		return fmt.Errorf("moving document: %w", err)
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
