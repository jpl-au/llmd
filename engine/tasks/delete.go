// delete.go handles soft-deleting and restoring tasks.

package tasks

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jpl-au/llmd/errs"

	"github.com/jpl-au/llmd/clock"

	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/task"
)

// Delete soft-deletes a task. The update and audit record are
// committed atomically.
func (t *Tasks) Delete(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return nil, err
	}

	now := clock.Now()
	_, err = t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		_, err := tx.ExecContext(ctx, `
			UPDATE tasks SET deleted_at = ? WHERE key = ? AND deleted_at IS NULL
		`, now, key)
		if err != nil {
			return nil, fmt.Errorf("deleting task: %w", err)
		}
		recordTx(ctx, tx, author, "deleted", key, tsk.Status, "")
		return nil, nil
	}).WithContext(ctx).Exec()
	if err != nil {
		return nil, err
	}

	if t.bus != nil {
		t.bus.Emit(ctx, events.Event{
			Type:      events.TaskDeleted,
			Path:      tsk.Path,
			Key:       key,
			Author:    author,
			Timestamp: now,
		})
	}

	return tsk, nil
}

// Restore undeletes a soft-deleted task. The update and audit record
// are committed atomically.
func (t *Tasks) Restore(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	// Read including deleted
	tsk, err := t.scanDeleted(ctx, key)
	if err != nil {
		return nil, err
	}

	_, err = t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks SET deleted_at = NULL WHERE key = ? AND deleted_at IS NOT NULL
		`, key)
		if err != nil {
			return nil, fmt.Errorf("restoring task: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return nil, fmt.Errorf("restoring task: %w", err)
		} else if n == 0 {
			return nil, errs.New(ErrNotFound, "task %s is not deleted.", key)
		}
		recordTx(ctx, tx, author, "restored", key, "", tsk.Status)
		return nil, nil
	}).WithContext(ctx).Exec()
	if err != nil {
		return nil, err
	}

	if t.bus != nil {
		t.bus.Emit(ctx, events.Event{
			Type:      events.TaskRestored,
			Path:      tsk.Path,
			Key:       key,
			Author:    author,
			Timestamp: clock.Now(),
		})
	}

	return tsk, nil
}
