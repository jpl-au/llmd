package entities

import (
	"context"

	"github.com/jpl-au/llmd/internal/clock"
)

// Delete soft-deletes an entity by key.
func (e *Entities) Delete(ctx context.Context, key string, opts DeleteOptions) error {
	now := clock.Now()

	qr, err := e.db.Query(`
		UPDATE entities SET deleted_at = ?
		WHERE key = ? AND deleted_at IS NULL
	`, now, key).WithContext(ctx).Exec()
	if err != nil {
		return err
	}

	rows, err := qr.SQLResult.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
