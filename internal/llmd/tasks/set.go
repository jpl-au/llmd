// set.go handles updating task metadata fields.

package tasks

import (
	"context"
	"database/sql"
	"fmt"
)

// Set updates task metadata.
func (t *Tasks) Set(ctx context.Context, key, author string, opts SetOptions) error {
	if err := t.ensure(); err != nil {
		return err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return err
	}

	if opts.Title != nil {
		old := tsk.Title
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET title = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Title, key)
		if err != nil {
			return fmt.Errorf("setting title: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:title", key, old, *opts.Title)
	}

	if opts.Priority != nil {
		old := fmt.Sprintf("%d", tsk.Priority)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET priority = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Priority, key)
		if err != nil {
			return fmt.Errorf("setting priority: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:priority", key, old, fmt.Sprintf("%d", *opts.Priority))
	}

	if opts.AssignedTo != nil {
		old := tsk.AssignedTo
		var v sql.NullString
		if *opts.AssignedTo != "" {
			v = sql.NullString{String: *opts.AssignedTo, Valid: true}
		}
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET assigned_to = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
		if err != nil {
			return fmt.Errorf("setting assigned_to: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:assigned_to", key, old, *opts.AssignedTo)
	}

	if opts.Branch != nil {
		old := tsk.Branch
		var v sql.NullString
		if *opts.Branch != "" {
			v = sql.NullString{String: *opts.Branch, Valid: true}
		}
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET branch = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
		if err != nil {
			return fmt.Errorf("setting branch: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:branch", key, old, *opts.Branch)
	}

	if opts.Position != nil {
		old := fmt.Sprintf("%d", tsk.Position)
		if err := t.reposition(ctx, key, tsk.Status, *opts.Position, author); err != nil {
			return err
		}
		_ = t.audit.Record(ctx, author, "edited:position", key, old, fmt.Sprintf("%d", *opts.Position))
	}

	if opts.Flag != "" {
		old := tsk.Flags
		flags := addFlag(tsk.Flags, opts.Flag)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
		if err != nil {
			return fmt.Errorf("setting flag: %w", err)
		}
		_ = t.audit.Record(ctx, author, "flagged", key, old, flags)
	}

	if opts.Unflag != "" {
		old := tsk.Flags
		flags := removeFlag(tsk.Flags, opts.Unflag)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
		if err != nil {
			return fmt.Errorf("removing flag: %w", err)
		}
		_ = t.audit.Record(ctx, author, "unflagged", key, old, flags)
	}

	return nil
}
