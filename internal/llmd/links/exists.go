package links

import (
	"context"
	"database/sql"
	"fmt"
)

// Exists checks if a link exists between two documents.
// If label is empty, checks for any link from→to.
// from and to can be document paths or keys.
func (l *Links) Exists(ctx context.Context, from, to string, opts ...Options) (bool, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	fromPath, err := l.resolvePath(ctx, from)
	if err != nil {
		return false, fmt.Errorf("resolving from: %w", err)
	}
	toPath, err := l.resolvePath(ctx, to)
	if err != nil {
		return false, fmt.Errorf("resolving to: %w", err)
	}

	var exists bool
	var row *sql.Row
	if opt.Label != "" {
		row, err = l.db.Query(`
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation = ?
				  AND json_extract(value, '$.to') = ?
				  AND json_extract(value, '$.label') = ?
				  AND deleted_at IS NULL
			)
		`, namespace, fromPath, toPath, opt.Label).WithContext(ctx).ReadRow()
	} else {
		row, err = l.db.Query(`
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation = ?
				  AND json_extract(value, '$.to') = ?
				  AND deleted_at IS NULL
			)
		`, namespace, fromPath, toPath).WithContext(ctx).ReadRow()
	}
	if err != nil {
		return false, err
	}
	if err := row.Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}
