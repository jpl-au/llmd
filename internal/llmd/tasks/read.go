// read.go handles reading and scanning individual tasks.

package tasks

import (
	"context"
	"database/sql"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// scanner is satisfied by both *sql.Row and *sql.Rows, allowing a
// single scan implementation for both single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// Read returns a task by key.
func (t *Tasks) Read(ctx context.Context, key string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	row, err := t.db.Query(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, depends_on, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE key = ? AND deleted_at IS NULL
	`, key).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	return t.scan(row)
}

// scanDeleted reads a task including soft-deleted ones.
func (t *Tasks) scanDeleted(ctx context.Context, key string) (*task.Task, error) {
	row, err := t.db.Query(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, depends_on, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE key = ?
		ORDER BY deleted_at DESC
		LIMIT 1
	`, key).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	return t.scan(row)
}

// scan reads a single task from a *sql.Row. Returns ErrNotFound when
// no row matches.
func (t *Tasks) scan(row *sql.Row) (*task.Task, error) {
	tsk, err := t.scanTask(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return tsk, err
}

// scanRow reads a single task from *sql.Rows (used in List iterations).
func (t *Tasks) scanRow(rows *sql.Rows) (*task.Task, error) {
	return t.scanTask(rows)
}

// scanTask scans a task from any source implementing the scanner
// interface. It handles the nullable columns (assigned_to, branch,
// flags, deleted_at) by scanning into sql.Null types and converting
// to Go zero values.
func (t *Tasks) scanTask(s scanner) (*task.Task, error) {
	var tsk task.Task
	var assignedTo, branch, flags, dependsOn sql.NullString
	var deletedAt sql.NullInt64

	err := s.Scan(
		&tsk.ID, &tsk.Key, &tsk.Title, &tsk.Status,
		&tsk.Priority, &tsk.Position, &assignedTo, &branch, &flags,
		&dependsOn, &tsk.Path, &tsk.Author, &tsk.Source, &tsk.CreatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		tsk.AssignedTo = assignedTo.String
	}
	if branch.Valid {
		tsk.Branch = branch.String
	}
	if flags.Valid {
		tsk.Flags = flags.String
	}
	if dependsOn.Valid {
		tsk.DependsOn = dependsOn.String
	}
	if deletedAt.Valid {
		tsk.DeletedAt = &deletedAt.Int64
	}
	return &tsk, nil
}
