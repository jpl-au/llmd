package documents

import (
	"context"
	"fmt"
)

// Purge permanently deletes all soft-deleted documents.
// Returns the number of rows deleted.
func (d *Documents) Purge(ctx context.Context) (int64, error) {
	qr, err := d.db.Query(`
		DELETE FROM content
		WHERE namespace = ? AND deleted_at IS NOT NULL
	`, namespace).WithContext(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("purging documents: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}
