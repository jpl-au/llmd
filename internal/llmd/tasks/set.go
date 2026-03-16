// set.go handles updating task metadata fields.

package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// Set updates task metadata. All changes are applied in a single
// transaction so a crash cannot leave a task partially updated.
func (t *Tasks) Set(ctx context.Context, key, author string, opts SetOptions) error {
	if err := t.ensure(); err != nil {
		return err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return err
	}

	_, err = t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		if opts.Title != nil {
			old := tsk.Title
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET title = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Title, key)
			if err != nil {
				return nil, fmt.Errorf("setting title: %w", err)
			}
			recordTx(ctx, tx, author, "edited:title", key, old, *opts.Title)
		}

		if opts.Priority != nil {
			old := fmt.Sprintf("%d", tsk.Priority)
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET priority = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Priority, key)
			if err != nil {
				return nil, fmt.Errorf("setting priority: %w", err)
			}
			recordTx(ctx, tx, author, "edited:priority", key, old, fmt.Sprintf("%d", *opts.Priority))
		}

		if opts.AssignedTo != nil {
			old := tsk.AssignedTo
			var v sql.NullString
			if *opts.AssignedTo != "" {
				v = sql.NullString{String: *opts.AssignedTo, Valid: true}
			}
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET assigned_to = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
			if err != nil {
				return nil, fmt.Errorf("setting assigned_to: %w", err)
			}
			recordTx(ctx, tx, author, "edited:assigned_to", key, old, *opts.AssignedTo)
		}

		if opts.Branch != nil {
			old := tsk.Branch
			var v sql.NullString
			if *opts.Branch != "" {
				v = sql.NullString{String: *opts.Branch, Valid: true}
			}
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET branch = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
			if err != nil {
				return nil, fmt.Errorf("setting branch: %w", err)
			}
			recordTx(ctx, tx, author, "edited:branch", key, old, *opts.Branch)
		}

		if opts.Position != nil {
			old := fmt.Sprintf("%d", tsk.Position)
			if err := t.repositionTx(ctx, tx, key, tsk.Status, *opts.Position); err != nil {
				return nil, err
			}
			recordTx(ctx, tx, author, "edited:position", key, old, fmt.Sprintf("%d", *opts.Position))
		}

		if opts.Flag != "" {
			old := tsk.Flags
			flags := addFlag(tsk.Flags, opts.Flag)
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
			if err != nil {
				return nil, fmt.Errorf("setting flag: %w", err)
			}
			recordTx(ctx, tx, author, "flagged", key, old, flags)
		}

		if opts.Unflag != "" {
			old := tsk.Flags
			flags := removeFlag(tsk.Flags, opts.Unflag)
			_, err := tx.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
			if err != nil {
				return nil, fmt.Errorf("removing flag: %w", err)
			}
			recordTx(ctx, tx, author, "unflagged", key, old, flags)
		}

		return nil, nil
	}).WithContext(ctx).Write()
	if err != nil {
		return err
	}

	if t.bus != nil {
		if err := t.bus.Emit(ctx, events.Event{
			Type:      events.TaskUpdated,
			Path:      tsk.Path,
			Key:       key,
			Author:    author,
			Timestamp: time.Now().UnixMilli(),
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
