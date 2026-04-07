package tags

import (
	"context"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// Remove removes a tag from a document (soft-delete).
// value can be a document path or key.
func (t *Tags) Remove(ctx context.Context, value, name string, opts Options) error {
	if err := Validate(name); err != nil {
		return err
	}
	if err := opts.Validate(); err != nil {
		return err
	}

	relation, err := t.resolvePath(ctx, value)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	qr, err := t.db.Query(`
		UPDATE entities SET deleted_at = ?
		WHERE namespace = ? AND relation = ? AND json_extract(value, '$.tag') = ?
		  AND deleted_at IS NULL
	`, now, namespace, relation, name).WithContext(ctx).Execute()

	if err != nil {
		return fmt.Errorf("removing tag: %w", err)
	}

	rows, err := qr.SQLResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	if t.bus != nil {
		if err := t.bus.Emit(ctx, events.Event{
			Type:      events.TagRemoved,
			Path:      relation,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"tag": name},
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
