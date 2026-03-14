// move.go handles moving tasks between columns and repositioning.

package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

// Move changes a task's status (column).
func (t *Tasks) Move(ctx context.Context, key, status, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return err
	}

	// Validate column
	cols, err := t.Columns(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(cols, status) {
		return fmt.Errorf("%w: %s", ErrInvalidCol, status)
	}

	// Spec gating: a task cannot leave the backlog until its document
	// contains real content beyond the title heading. This enforces
	// that every active task has a written spec describing the work.
	if tsk.Status == "backlog" && status != "backlog" {
		specced, err := t.hasSpec(ctx, tsk.Path)
		if err != nil {
			return err
		}
		if !specced {
			return ErrNoSpec
		}
	}

	oldStatus := tsk.Status

	_, err = t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		// Next position in target column
		var maxPos int
		err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(position), -1) FROM tasks
			WHERE status = ? AND deleted_at IS NULL
		`, status).Scan(&maxPos)
		if err != nil {
			return nil, fmt.Errorf("getting position: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE tasks SET status = ?, position = ? WHERE key = ? AND deleted_at IS NULL
		`, status, maxPos+1, key)
		if err != nil {
			return nil, fmt.Errorf("moving task: %w", err)
		}

		recordTx(ctx, tx, author, "moved", key, oldStatus, status)
		return nil, nil
	}).WithContext(ctx).Write()

	return err
}

// repositionTx moves a task to a specific position within its column,
// renumbering other tasks to maintain order. All updates run on the
// provided transaction.
func (t *Tasks) repositionTx(ctx context.Context, tx *sql.Tx, key, status string, pos int) error {
	// Get all tasks in this column, ordered by position
	rows, err := tx.QueryContext(ctx, `
		SELECT key FROM tasks
		WHERE status = ? AND deleted_at IS NULL
		ORDER BY position ASC, created_at ASC
	`, status)
	if err != nil {
		return fmt.Errorf("listing column: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Remove the target key from the list
	var others []string
	for _, k := range keys {
		if k != key {
			others = append(others, k)
		}
	}

	// Clamp position
	if pos < 0 {
		pos = 0
	}
	if pos > len(others) {
		pos = len(others)
	}

	// Insert at desired position
	reordered := make([]string, 0, len(others)+1)
	reordered = append(reordered, others[:pos]...)
	reordered = append(reordered, key)
	reordered = append(reordered, others[pos:]...)

	// Update all positions
	for i, k := range reordered {
		_, err := tx.ExecContext(ctx, `
			UPDATE tasks SET position = ? WHERE key = ? AND deleted_at IS NULL
		`, i, k)
		if err != nil {
			return fmt.Errorf("renumbering: %w", err)
		}
	}

	return nil
}

// hasSpec checks whether a task's document has real content beyond
// the title heading. A document that is missing, empty, or contains
// only a single "# Title" line does not count — the spec must
// describe the work (context, acceptance criteria, etc.) before the
// task can leave the backlog.
func (t *Tasks) hasSpec(ctx context.Context, path string) (bool, error) {
	doc, err := t.docs.Read(ctx, path)
	if err != nil {
		return false, nil // No document = no spec
	}
	content := strings.TrimSpace(doc.Content)
	if idx := strings.Index(content, "\n"); idx >= 0 {
		after := strings.TrimSpace(content[idx:])
		return after != "", nil
	}
	// Single line = just the heading
	return false, nil
}
