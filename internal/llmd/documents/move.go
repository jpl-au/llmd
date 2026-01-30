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
	if ok, err := d.Exists(ctx, dst); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("destination already exists: %s", dst)
	}

	// Get the latest version info before moving (for the event)
	var key string
	var version int
	err := d.db.QueryRowContext(ctx, `
		SELECT key, version FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, src).Scan(&key, &version)
	if err != nil {
		return ErrNotFound
	}

	result, err := d.db.ExecContext(ctx, `
		UPDATE content SET path = ?
		WHERE namespace = ? AND path = ?
	`, dst, namespace, src)
	if err != nil {
		return fmt.Errorf("moving document: %w", err)
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
