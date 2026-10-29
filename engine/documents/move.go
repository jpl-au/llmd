package documents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/errs"

	"github.com/jpl-au/llmd/clock"

	"github.com/jpl-au/llmd/engine/fts"
	"github.com/jpl-au/llmd/pkg/events"
)

// Move renames a document from src to dst.
// All versions are moved to the new path. src must be live (ErrDeleted,
// ErrNotFound otherwise) and dst must have no versions (ErrExists).
func (d *Documents) Move(ctx context.Context, src, dst string, opts MoveOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	type moved struct {
		key     string
		version int
	}
	result, err := d.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		if err := CheckTx(ctx, tx, src); err != nil {
			return nil, err
		}
		// The destination must be free. A deleted document there still
		// owns its versions: merging two histories under one path would
		// make neither make sense.
		switch err := CheckTx(ctx, tx, dst); {
		case err == nil:
			return nil, errs.New(ErrExists, "%s already exists.", dst)
		case errors.Is(err, ErrDeleted):
			return nil, errs.New(ErrExists, "%s is a deleted document. Restore it, or vacuum to purge it.", dst)
		case !errors.Is(err, ErrNotFound):
			return nil, err
		}

		// The row the index holds for src is the latest live version,
		// which is also what the event reports.
		before, err := fts.Live(ctx, tx, src)
		if err != nil {
			return nil, err
		}
		var m moved
		if err := tx.QueryRowContext(ctx, `
			SELECT key, version FROM content WHERE id = ?
		`, before.ID).Scan(&m.key, &m.version); err != nil {
			return nil, fmt.Errorf("reading document version: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE content SET path = ?
			WHERE namespace = ? AND path = ?
		`, dst, namespace, src); err != nil {
			return nil, fmt.Errorf("moving document: %w", err)
		}

		return m, fts.Sync(ctx, tx, before, dst)
	}).WithContext(ctx).Exec()
	if err != nil {
		return err
	}
	m := result.Value.(moved)

	if d.bus != nil {
		d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentMoved,
			Path:      dst,
			Key:       m.key,
			Version:   m.version,
			Author:    opts.Author,
			Timestamp: clock.Now(),
			Metadata: map[string]any{
				"old_path": src,
			},
		})
	}

	return nil
}
