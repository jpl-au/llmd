package entities

import (
	"context"
	"time"
)

// Delete soft-deletes an entity by key.
func (e *Entities) Delete(ctx context.Context, key string, opts DeleteOptions) error {
	now := time.Now().UnixMilli()

	result, err := e.db.ExecContext(ctx, `
		UPDATE entities SET deleted_at = ?
		WHERE key = ? AND deleted_at IS NULL
	`, now, key)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
