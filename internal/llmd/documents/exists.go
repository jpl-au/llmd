package documents

import (
	"context"
	"database/sql"
)

// Exists checks if a document exists at the given path or key.
// Returns true if the document exists and is not deleted.
func (d *Documents) Exists(ctx context.Context, value string) (bool, error) {
	// Try by key first (9-char identifier)
	if len(value) == 9 {
		var exists bool
		err := d.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM content
				WHERE namespace = ? AND key = ? AND deleted_at IS NULL
			)
		`, namespace, value).Scan(&exists)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}

	// Try by path
	var exists bool
	err := d.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM content
			WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		)
	`, namespace, value).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}

	return exists, nil
}
