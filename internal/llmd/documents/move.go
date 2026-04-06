package documents

import (
	"context"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// Move renames a document from src to dst.
// All versions are moved to the new path.
func (d *Documents) Move(ctx context.Context, src, dst string, opts MoveOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	// Check destination doesn't exist
	ok, err := d.Exists(ctx, dst)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("destination already exists: %s", dst)
	}

	// Get the latest version info before moving (for the event)
	var key string
	var version int
	row, err := d.db.Query(`
		SELECT key, version FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, src).WithContext(ctx).ReadRow()
	if err != nil {
		return ErrNotFound
	}
	if err := row.Scan(&key, &version); err != nil {
		return ErrNotFound
	}

	qr, err := d.db.Query(`
		UPDATE content SET path = ?
		WHERE namespace = ? AND path = ?
	`, dst, namespace, src).WithContext(ctx).Execute()
	if err != nil {
		return fmt.Errorf("moving document: %w", err)
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
		now := time.Now().UnixMilli()
		if err := d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentMoved,
			Path:      dst,
			Key:       key,
			Version:   version,
			Author:    opts.Author,
			Timestamp: now,
			Metadata: map[string]any{
				"old_path": src,
			},
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
