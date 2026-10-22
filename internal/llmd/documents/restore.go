package documents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/internal/clock"

	"github.com/jpl-au/llmd/pkg/events"
)

// Restore restores a soft-deleted document at the given path.
// All versions of the document are restored.
func (d *Documents) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	// Get the latest version info before restoring (for the event)
	var key string
	var version int
	row, err := d.db.Query(`
		SELECT key, version FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NOT NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("finding deleted document: %w", err)
	}
	if err := row.Scan(&key, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("finding deleted document: %w", err)
	}

	qr, err := d.db.Query(`
		UPDATE content SET deleted_at = NULL
		WHERE namespace = ? AND path = ? AND deleted_at IS NOT NULL
	`, namespace, path).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("restoring document: %w", err)
	}

	rows, err := qr.SQLResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Emit event
	if d.bus != nil {
		now := clock.Now()
		if err := d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentRestored,
			Path:      path,
			Key:       key,
			Version:   version,
			Author:    opts.Author,
			Timestamp: now,
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
