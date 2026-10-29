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

// Restore restores a soft-deleted document at the given path.
// All versions of the document are restored. Returns ErrNotDeleted for
// a live document and ErrNotFound when the path has no versions.
func (d *Documents) Restore(ctx context.Context, path string, opts RestoreOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	type restored struct {
		key     string
		version int
	}
	result, err := d.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		switch err := CheckTx(ctx, tx, path); {
		case err == nil:
			return nil, errs.New(ErrNotDeleted, "%s is not deleted.", path)
		case !errors.Is(err, ErrDeleted):
			return nil, err
		}

		// The latest version, for the event.
		var r restored
		if err := tx.QueryRowContext(ctx, `
			SELECT key, version FROM content
			WHERE namespace = ? AND path = ?
			ORDER BY version DESC LIMIT 1
		`, namespace, path).Scan(&r.key, &r.version); err != nil {
			return nil, fmt.Errorf("finding deleted document: %w", err)
		}

		before, err := fts.Live(ctx, tx, path)
		if err != nil {
			return nil, err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE content SET deleted_at = NULL
			WHERE namespace = ? AND path = ? AND deleted_at IS NOT NULL
		`, namespace, path); err != nil {
			return nil, fmt.Errorf("restoring document: %w", err)
		}

		return r, fts.Sync(ctx, tx, before, path)
	}).WithContext(ctx).Exec()
	if err != nil {
		return err
	}
	r := result.Value.(restored)

	if d.bus != nil {
		d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentRestored,
			Path:      path,
			Key:       r.key,
			Version:   r.version,
			Author:    opts.Author,
			Timestamp: clock.Now(),
		})
	}

	return nil
}
