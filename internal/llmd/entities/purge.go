package entities

import "context"

// Purge permanently removes soft-deleted entities.
func (e *Entities) Purge(ctx context.Context) (int64, error) {
	result, err := e.db.ExecContext(ctx, `
		DELETE FROM entities WHERE deleted_at IS NOT NULL
	`)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
