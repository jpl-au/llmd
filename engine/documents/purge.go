package documents

import (
	"context"
	"fmt"
)

// PurgeOptions narrows a purge. Before keeps anything soft-deleted at or
// after that Unix-millisecond time (0 purges every soft-deleted
// document). DryRun reports what would be purged without changing the
// database.
type PurgeOptions struct {
	Before int64
	DryRun bool
}

// Purge permanently deletes soft-deleted documents, every version of
// each. Returns the paths purged, or that would be purged under DryRun,
// in path order.
func (d *Documents) Purge(ctx context.Context, opts PurgeOptions) ([]string, error) {
	paths, err := d.deletedPaths(ctx, opts.Before)
	if err != nil || opts.DryRun || len(paths) == 0 {
		return paths, err
	}

	_, err = d.db.Query(`
		DELETE FROM content
		WHERE namespace = ? AND deleted_at IS NOT NULL
		AND (? = 0 OR deleted_at < ?)
	`, namespace, opts.Before, opts.Before).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("purging documents: %w", err)
	}
	return paths, nil
}

// deletedPaths lists the distinct paths of soft-deleted documents,
// optionally only those deleted before the cutoff.
func (d *Documents) deletedPaths(ctx context.Context, before int64) ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT path FROM content
		WHERE namespace = ? AND deleted_at IS NOT NULL
		AND (? = 0 OR deleted_at < ?)
		ORDER BY path
	`, namespace, before, before).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing deleted documents: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning deleted document: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
