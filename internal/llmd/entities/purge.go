package entities

import "context"

// Purge permanently removes soft-deleted entities.
func (e *Entities) Purge(ctx context.Context) (int64, error) {
	qr, err := e.db.Query(`
		DELETE FROM entities WHERE deleted_at IS NOT NULL
	`).WithContext(ctx).Execute()
	if err != nil {
		return 0, err
	}

	return qr.SQLResult.RowsAffected()
}
