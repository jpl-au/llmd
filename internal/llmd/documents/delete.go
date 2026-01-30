package documents

import (
	"context"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// Delete soft-deletes a document at the given path.
// All versions of the document are marked as deleted.
func (d *Documents) Delete(ctx context.Context, path string, opts DeleteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	result, err := d.db.ExecContext(ctx, `
		UPDATE content SET deleted_at = ?
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
	`, now, namespace, path)
	if err != nil {
		return fmt.Errorf("deleting document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Emit event
	if d.bus != nil {
		if err := d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentDeleted,
			Path:      path,
			Author:    opts.Author,
			Timestamp: now,
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
