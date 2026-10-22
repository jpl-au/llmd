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
	`, namespace).WithContext(ctx).Exec()
	if err != nil {
		return 0, fmt.Errorf("purging links: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}

// PurgeOrphans permanently deletes links where either endpoint no
// longer has any version in the store. Links touching a soft-deleted
// document are kept (its rows still exist) so that restoring the
// document brings them back; only a hard purge orphans them.
func (l *Links) PurgeOrphans(ctx context.Context) (int64, error) {
	qr, err := l.db.Query(`
		DELETE FROM entities
		WHERE namespace = ?
		AND (
			NOT EXISTS (
				SELECT 1 FROM content c
				WHERE c.namespace = 'core:document' AND c.path = entities.relation
			)
			OR NOT EXISTS (
				SELECT 1 FROM content c
				WHERE c.namespace = 'core:document' AND c.path = json_extract(entities.value, '$.to')
			)
		)
	`, namespace).WithContext(ctx).Exec()
	if err != nil {
		return 0, fmt.Errorf("purging orphaned links: %w", err)
	}

	return qr.SQLResult.RowsAffected()
}
