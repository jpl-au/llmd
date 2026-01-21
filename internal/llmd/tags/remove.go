package tags

import (
	"context"
	"fmt"
	"time"
)

// Remove removes a tag from a document (soft-delete).
// value can be a document path or 9-char key.
func (t *Tags) Remove(ctx context.Context, value, name string, opts Options) error {
	if err := Validate(name); err != nil {
		return err
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, value)
	if err != nil {
		return err
	}
	path := doc.Path

	now := time.Now().UnixMilli()

	result, err := t.db.ExecContext(ctx, `
		UPDATE entities SET deleted_at = ?
		WHERE namespace = ? AND path = ? AND json_extract(value, '$.tag') = ?
		  AND deleted_at IS NULL
	`, now, namespace, path, name)

	if err != nil {
		return fmt.Errorf("removing tag: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
