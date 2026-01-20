package tags

import (
	"context"
	"fmt"
)

// Purge permanently deletes all soft-deleted tags.
// Returns the number of rows deleted.
func (t *Tags) Purge(ctx context.Context) (int64, error) {
	result, err := t.db.ExecContext(ctx, `
		DELETE FROM entities
		WHERE namespace = ? AND deleted_at IS NOT NULL
	`, namespace)
	if err != nil {
		return 0, fmt.Errorf("purging tags: %w", err)
	}

	return result.RowsAffected()
}
