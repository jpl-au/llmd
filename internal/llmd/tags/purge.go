package tags

import (
	"context"
	"fmt"
)

// Purge permanently deletes all soft-deleted tags.
// Returns the number of rows deleted.
func (t *Tags) Purge(ctx context.Context) (int64, error) {
	qr, err := t.db.Query(`
		DELETE FROM entities
		WHERE namespace = ? AND deleted_at IS NOT NULL
	`, namespace).WithContext(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("purging tags: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}
