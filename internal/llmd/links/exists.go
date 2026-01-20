package links

import (
	"context"
	"fmt"
)

// Exists checks if a link exists between two documents.
// If label is empty, checks for any link from→to.
// fromPathOrKey and toPathOrKey can be document paths or 9-char keys.
func (l *Links) Exists(ctx context.Context, fromPathOrKey, toPathOrKey string, opts ...Options) (bool, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Resolve both documents
	fromDoc, err := l.docs.Resolve(ctx, fromPathOrKey)
	if err != nil {
		return false, fmt.Errorf("resolving from: %w", err)
	}
	toDoc, err := l.docs.Resolve(ctx, toPathOrKey)
	if err != nil {
		return false, fmt.Errorf("resolving to: %w", err)
	}

	var exists bool
	if opt.Label != "" {
		err = l.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND path = ?
				  AND json_extract(value, '$.to') = ?
				  AND json_extract(value, '$.label') = ?
				  AND deleted_at IS NULL
			)
		`, namespace, fromDoc.Path, toDoc.Path, opt.Label).Scan(&exists)
	} else {
		err = l.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND path = ?
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
