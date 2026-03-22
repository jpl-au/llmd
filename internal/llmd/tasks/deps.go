// deps.go provides dependency chain queries.

package tasks

import (
	"context"
	"fmt"
	"slices"

	"github.com/jpl-au/llmd/pkg/model/task"
)

// Dep returns the single task this task depends on, or nil if none.
func (t *Tasks) Dep(ctx context.Context, key string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	tsk, err := t.Read(ctx, key)
	if err != nil {
		return nil, err
	}
	if tsk.DependsOn == "" {
		return nil, nil
	}
	return t.Read(ctx, tsk.DependsOn)
}

// Dependents returns tasks that directly depend on the given task.
func (t *Tasks) Dependents(ctx context.Context, key string) ([]*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	rows, err := t.db.Query(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, depends_on, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE depends_on = ? AND deleted_at IS NULL
	`, key).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing dependents: %w", err)
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

// Chain returns the full dependency chain starting from key, walked
// backwards until a task with no dependency is reached. Returns tasks
// in dependency order (deepest dependency first, key last).
func (t *Tasks) Chain(ctx context.Context, key string) ([]*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	var chain []*task.Task
	seen := map[string]bool{}

	current := key
	for {
		tsk, err := t.Read(ctx, current)
		if err != nil {
			return nil, err
		}
		chain = append(chain, tsk)
		seen[current] = true

		if tsk.DependsOn == "" {
			break
		}
		if seen[tsk.DependsOn] {
			return nil, fmt.Errorf("%w: %s", ErrCycle, tsk.DependsOn)
		}
		current = tsk.DependsOn
	}

	slices.Reverse(chain)
	return chain, nil
}

// Ready returns true if the full dependency chain is satisfied
// (every dependency has status "done").
func (t *Tasks) Ready(ctx context.Context, key string) (bool, error) {
	if err := t.ensure(); err != nil {
		return false, err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return false, err
	}

	for tsk.DependsOn != "" {
		dep, err := t.Read(ctx, tsk.DependsOn)
		if err != nil {
			return false, err
		}
		if dep.Status != "done" {
			return false, nil
		}
		tsk = dep
	}

	return true, nil
}
