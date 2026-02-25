// columns.go manages board columns: list, add, remove, reorder.

package tasks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// Columns returns the board columns in order.
func (t *Tasks) Columns(ctx context.Context) ([]string, error) {
	return t.readColumns(ctx)
}

// AddColumn adds a new column. If after is empty, appends to the end.
func (t *Tasks) AddColumn(ctx context.Context, name, after, author string) error {
	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColExists, name)
	}

	if after == "" {
		cols = append(cols, name)
	} else {
		idx := slices.Index(cols, after)
		if idx < 0 {
			return fmt.Errorf("%w: %s", ErrColNotFound, after)
		}
		cols = append(cols[:idx+1], append([]string{name}, cols[idx+1:]...)...)
	}

	return t.writeColumns(ctx, cols, author)
}

// RemoveColumn removes a column if it has no tasks.
func (t *Tasks) RemoveColumn(ctx context.Context, name, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}

	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}

	// Check for tasks in this column
	var count int
	err = t.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE status = ? AND deleted_at IS NULL
	`, name).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking column: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %s (%d tasks)", ErrColNotEmpty, name, count)
	}

	var filtered []string
	for _, c := range cols {
		if c != name {
			filtered = append(filtered, c)
		}
	}

	return t.writeColumns(ctx, filtered, author)
}

// MoveColumn reorders a column to be after another column.
func (t *Tasks) MoveColumn(ctx context.Context, name, after, author string) error {
	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}
	if !slices.Contains(cols, after) {
		return fmt.Errorf("%w: %s", ErrColNotFound, after)
	}

	// Remove name from current position
	var filtered []string
	for _, c := range cols {
		if c != name {
			filtered = append(filtered, c)
		}
	}

	// Insert after target
	idx := slices.Index(filtered, after)
	result := append(filtered[:idx+1], append([]string{name}, filtered[idx+1:]...)...)

	return t.writeColumns(ctx, result, author)
}

// readColumns reads the board columns from the entity.
func (t *Tasks) readColumns(ctx context.Context) ([]string, error) {
	exists, err := t.entities.ExistsInNamespace(ctx, boardNamespace, "")
	if err != nil {
		return nil, err
	}
	if !exists {
		return DefaultColumns, nil
	}

	ents, err := t.entities.List(ctx, boardNamespace, entities.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(ents) == 0 {
		return DefaultColumns, nil
	}

	// Parse the JSON value
	return parseColumns(ents[0].Value)
}

// writeColumns updates the board entity with new columns.
// Soft-deletes the old entity and creates a new one (insert-only).
func (t *Tasks) writeColumns(ctx context.Context, cols []string, author string) error {
	// Soft-delete existing board entity
	ents, err := t.entities.List(ctx, boardNamespace, entities.ListOptions{})
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := t.entities.Delete(ctx, e.Key, entities.DeleteOptions{
			Origin: core.Origin{Author: author, Source: "cli"},
		}); err != nil {
			return err
		}
	}

	// Write new one
	value := formatColumns(cols)
	_, err = t.entities.Write(ctx, boardNamespace, value, entities.WriteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
	return err
}

// parseColumns extracts the columns array from the board entity JSON.
func parseColumns(value string) ([]string, error) {
	// Simple parsing — the value is {"columns":["a","b","c"]}
	// Use strings rather than encoding/json to avoid allocations for
	// this hot path in list operations.
	start := strings.Index(value, "[")
	end := strings.LastIndex(value, "]")
	if start < 0 || end < 0 || end <= start {
		return DefaultColumns, nil
	}

	inner := value[start+1 : end]
	if inner == "" {
		return nil, nil
	}

	var cols []string
	for part := range strings.SplitSeq(inner, ",") {
		col := strings.Trim(strings.TrimSpace(part), `"`)
		if col != "" {
			cols = append(cols, col)
		}
	}
	return cols, nil
}

// formatColumns serialises the column list to the JSON format stored in
// the board entity: {"columns":["backlog","up-next",...]}.
func formatColumns(cols []string) string {
	var quoted []string
	for _, c := range cols {
		quoted = append(quoted, `"`+c+`"`)
	}
	return `{"columns":[` + strings.Join(quoted, ",") + `]}`
}
