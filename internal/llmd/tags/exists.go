package tags

import (
	"context"
)

// Exists checks if a tag exists on a document.
// pathOrKey can be a document path or 9-char key.
func (t *Tags) Exists(ctx context.Context, pathOrKey, tagName string) (bool, error) {
	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, pathOrKey)
	if err != nil {
		return false, err
	}

	var exists bool
	err = t.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM entities
			WHERE namespace = ? AND path = ? AND json_extract(value, '$.tag') = ?
			  AND deleted_at IS NULL
		)
	`, namespace, doc.Path, tagName).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}
