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

// Delete soft-deletes a document at the given path. Returns ErrDeleted
// when it is already deleted and ErrNotFound when the path has none.
// All versions of the document are marked as deleted and the document
// leaves the search index in the same transaction.
func (d *Documents) Delete(ctx context.Context, path string, opts DeleteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	now := clock.Now()

	_, err := d.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		if err := CheckTx(ctx, tx, path); err != nil {
			if errors.Is(err, ErrDeleted) {
				return nil, errs.New(ErrDeleted, "%s is already deleted.", path)
			}
			return nil, err
		}
		before, err := fts.Live(ctx, tx, path)
		if err != nil {
			return nil, err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE content SET deleted_at = ?
			WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		`, now, namespace, path); err != nil {
			return nil, fmt.Errorf("deleting document: %w", err)
		}

		return nil, fts.Sync(ctx, tx, before, path)
	}).WithContext(ctx).Exec()
	if err != nil {
		return err
	}

	if d.bus != nil {
		d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentDeleted,
			Path:      path,
			Author:    opts.Author,
			Timestamp: now,
		})
	}

	return nil
}
