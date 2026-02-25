// delete.go handles soft-deleting and restoring tasks.

package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// Delete soft-deletes a task.
func (t *Tasks) Delete(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	_, err = t.db.ExecContext(ctx, `
		UPDATE tasks SET deleted_at = ? WHERE key = ? AND deleted_at IS NULL
	`, now, key)
	if err != nil {
		return nil, fmt.Errorf("deleting task: %w", err)
	}

	_ = t.audit.Record(ctx, author, "deleted", key, tsk.Status, "")
	return tsk, nil
}

// Restore undeletes a soft-deleted task.
func (t *Tasks) Restore(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	// Read including deleted
	tsk, err := t.scanDeleted(ctx, key)
	if err != nil {
		return nil, err
	}

	_, err = t.db.ExecContext(ctx, `
		UPDATE tasks SET deleted_at = NULL WHERE key = ? AND deleted_at IS NOT NULL
	`, key)
	if err != nil {
		return nil, fmt.Errorf("restoring task: %w", err)
	}

	_ = t.audit.Record(ctx, author, "restored", key, "", tsk.Status)
	return tsk, nil
}
