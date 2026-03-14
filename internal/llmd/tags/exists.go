package tags

import (
	"context"
)

// Exists checks if a tag exists on a document.
// value can be a document path or key.
func (t *Tags) Exists(ctx context.Context, value, name string) (bool, error) {
	if err := Validate(name); err != nil {
		return false, err
	}

	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, value)
	if err != nil {
		return false, err
	}

	var exists bool
	row, err := t.db.Query(`
		SELECT EXISTS(
			SELECT 1 FROM entities
			WHERE namespace = ? AND relation = ? AND json_extract(value, '$.tag') = ?
			  AND deleted_at IS NULL
		)
	`, namespace, doc.Path, name).WithContext(ctx).ReadRow()
	if err != nil {
		return false, err
	}
	if err := row.Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}
