// list.go handles listing tasks with filters.

package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// List returns all non-deleted tasks matching the filter criteria.
// Results are ordered by position then creation time. When no filters
// are set, returns every active task on the board.
func (t *Tasks) List(ctx context.Context, opts ListOptions) ([]*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE deleted_at IS NULL
	`)

	if opts.Status != "" {
		query.WriteString(" AND status = ?")
		args = append(args, opts.Status)
	}
	if opts.AssignedTo != "" {
		query.WriteString(" AND assigned_to = ?")
		args = append(args, opts.AssignedTo)
	}
	if opts.Priority > 0 {
		query.WriteString(" AND priority = ?")
		args = append(args, opts.Priority)
	}
	if opts.Branch != "" {
		query.WriteString(" AND branch = ?")
		args = append(args, opts.Branch)
	}

	query.WriteString(" ORDER BY position ASC, created_at ASC")

	rows, err := t.db.Query(query.String(), args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*task.Task
	for rows.Next() {
		tsk, err := t.scanRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, tsk)
	}
	return tasks, rows.Err()
}
