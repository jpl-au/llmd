package links

import (
	"context"
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

	// Resolve both documents
	fromDoc, err := l.docs.Resolve(ctx, from)
	if err != nil {
		return false, fmt.Errorf("resolving from: %w", err)
	}
	toDoc, err := l.docs.Resolve(ctx, to)
	if err != nil {
		return false, fmt.Errorf("resolving to: %w", err)
	}

	var exists bool
	if opt.Label != "" {
		err = l.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation = ?
				  AND json_extract(value, '$.to') = ?
				  AND json_extract(value, '$.label') = ?
				  AND deleted_at IS NULL
			)
		`, namespace, fromDoc.Path, toDoc.Path, opt.Label).Scan(&exists)
	} else {
		err = l.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation = ?
				  AND json_extract(value, '$.to') = ?
				  AND deleted_at IS NULL
			)
		`, namespace, fromDoc.Path, toDoc.Path).Scan(&exists)
	}

	if err != nil {
		return false, err
	}

	return exists, nil
}
