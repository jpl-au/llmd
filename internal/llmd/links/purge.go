package links

import (
	"context"
	"fmt"
)

// PurgeOptions narrows a purge. Before keeps anything soft-deleted at or
// after that Unix-millisecond time and treats documents deleted after it
// as still present (0 applies no cutoff). DryRun counts what would be
// purged without changing the database.
type PurgeOptions struct {
	Before int64
	DryRun bool
}

// purgeWhere selects links that are soft-deleted (before the cutoff,
// when one is set) or orphaned: links where either endpoint has no live
// version and no soft-deleted version newer than the cutoff. Links on a
// merely soft-deleted document survive so that restoring the document
// brings them back; only a hard purge of the document orphans them.
// Arguments: namespace, then the cutoff six times.
const purgeWhere = `
	namespace = ?
	AND (
		(deleted_at IS NOT NULL AND (? = 0 OR deleted_at < ?))
		OR NOT EXISTS (
			SELECT 1 FROM content c
			WHERE c.namespace = 'core:document' AND c.path = entities.relation
			AND (c.deleted_at IS NULL OR (? <> 0 AND c.deleted_at >= ?))
		)
		OR NOT EXISTS (
			SELECT 1 FROM content c
			WHERE c.namespace = 'core:document' AND c.path = json_extract(entities.value, '$.to')
			AND (c.deleted_at IS NULL OR (? <> 0 AND c.deleted_at >= ?))
		)
	)
`

// Purge permanently deletes soft-deleted and orphaned links. Returns the
// number of rows deleted, or that would be deleted under DryRun.
func (l *Links) Purge(ctx context.Context, opts PurgeOptions) (int64, error) {
	b := opts.Before
	args := []any{namespace, b, b, b, b, b, b}

	if opts.DryRun {
		var n int64
		row, err := l.db.Query("SELECT COUNT(*) FROM entities WHERE "+purgeWhere, args...).WithContext(ctx).ReadRow()
		if err != nil {
			return 0, fmt.Errorf("counting purgeable links: %w", err)
		}
		if err := row.Scan(&n); err != nil {
			return 0, fmt.Errorf("counting purgeable links: %w", err)
		}
		return n, nil
	}

	qr, err := l.db.Query("DELETE FROM entities WHERE "+purgeWhere, args...).WithContext(ctx).Exec()
	if err != nil {
		return 0, fmt.Errorf("purging links: %w", err)
	}
	return qr.SQLResult.RowsAffected()
}
