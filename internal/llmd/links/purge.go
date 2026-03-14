package links

import (
	"context"
	"fmt"
)

// Purge permanently deletes all soft-deleted links.
// Returns the number of rows deleted.
func (l *Links) Purge(ctx context.Context) (int64, error) {
	qr, err := l.db.Query(`
		DELETE FROM entities
		WHERE namespace = ? AND deleted_at IS NOT NULL
	`, namespace).WithContext(ctx).Execute()
	if err != nil {
		return 0, fmt.Errorf("purging links: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}
