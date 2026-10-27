// list.go handles listing tasks with filters.

package tasks

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// List returns all non-deleted tasks matching the filter criteria in
// board order: by column (as the board defines them), then within the
// column by the configured Order (priority first by default, then
// position, creation time and id). When no filters are set, returns
// every active task on the board.
func (t *Tasks) List(ctx context.Context, opts ListOptions) ([]*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, depends_on, path, author, source, created_at, deleted_at
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
	if opts.SinceMS > 0 {
		query.WriteString(" AND created_at > ?")
		args = append(args, opts.SinceMS)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	cols, err := t.Columns(ctx)
	if err != nil {
		return nil, err
	}
	rank := make(map[string]int, len(cols))
	for i, c := range cols {
		rank[c] = i
	}
	column := func(status string) int {
		if i, ok := rank[status]; ok {
			return i
		}
		return len(cols) // unknown columns sort last
	}
	slices.SortStableFunc(tasks, func(a, b *task.Task) int {
		if c := cmp.Compare(column(a.Status), column(b.Status)); c != 0 {
			return c
		}
		return t.compare(a, b)
	})
	return tasks, nil
}
