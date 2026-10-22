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
	`, namespace).WithContext(ctx).Exec()
	if err != nil {
		return 0, fmt.Errorf("purging tags: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}

// PurgeOrphans permanently deletes tags whose document no longer has
// any version in the store. Tags on a soft-deleted document are kept
// (its rows still exist) so that restoring the document brings them
// back; only a hard purge of the document orphans them.
func (t *Tags) PurgeOrphans(ctx context.Context) (int64, error) {
	qr, err := t.db.Query(`
		DELETE FROM entities
		WHERE namespace = ?
		AND NOT EXISTS (
			SELECT 1 FROM content c
			WHERE c.namespace = 'core:document' AND c.path = entities.relation
		)
	`, namespace).WithContext(ctx).Exec()
	if err != nil {
		return 0, fmt.Errorf("purging orphaned tags: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}
