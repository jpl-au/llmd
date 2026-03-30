// columns.go manages board columns and pipeline configuration.

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// Board holds the column list.
type Board struct {
	Columns []string `json:"columns"`
}

// Columns returns the board columns in order.
func (t *Tasks) Columns(ctx context.Context) ([]string, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	b, err := t.readBoard(ctx)
	if err != nil {
		return nil, err
	}
	return b.Columns, nil
}

// AddColumn adds a new column. If after is empty, appends to the end.
func (t *Tasks) AddColumn(ctx context.Context, name, after, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}
	b, err := t.readBoard(ctx)
	if err != nil {
		return err
	}

	if slices.Contains(b.Columns, name) {
		return fmt.Errorf("%w: %s", ErrColExists, name)
	}

	if after == "" {
		b.Columns = append(b.Columns, name)
	} else {
		idx := slices.Index(b.Columns, after)
		if idx < 0 {
			return fmt.Errorf("%w: %s", ErrColNotFound, after)
		}
		b.Columns = append(b.Columns[:idx+1], append([]string{name}, b.Columns[idx+1:]...)...)
	}

	return t.writeBoard(ctx, b, author)
}

// RemoveColumn removes a column if it has no tasks.
func (t *Tasks) RemoveColumn(ctx context.Context, name, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}

	b, err := t.readBoard(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(b.Columns, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}

	var count int
	row, err := t.db.Query(`
		SELECT COUNT(*) FROM tasks WHERE status = ? AND deleted_at IS NULL
	`, name).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("checking column: %w", err)
	}
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("checking column: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %s (%d tasks)", ErrColNotEmpty, name, count)
	}

	var filtered []string
	for _, c := range b.Columns {
		if c != name {
			filtered = append(filtered, c)
		}
	}
	b.Columns = filtered

	return t.writeBoard(ctx, b, author)
}

// MoveColumn reorders a column to be after another column.
func (t *Tasks) MoveColumn(ctx context.Context, name, after, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}
	b, err := t.readBoard(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(b.Columns, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}
	if !slices.Contains(b.Columns, after) {
		return fmt.Errorf("%w: %s", ErrColNotFound, after)
	}

	var filtered []string
	for _, c := range b.Columns {
		if c != name {
			filtered = append(filtered, c)
		}
	}

	idx := slices.Index(filtered, after)
	b.Columns = append(filtered[:idx+1], append([]string{name}, filtered[idx+1:]...)...)

	return t.writeBoard(ctx, b, author)
}

// readBoard reads the latest board row. Returns default columns if
// the table is empty.
func (t *Tasks) readBoard(ctx context.Context) (*Board, error) {
	row, err := t.db.Query(
		`SELECT value FROM board ORDER BY created_at DESC, id DESC LIMIT 1`,
	).WithContext(ctx).ReadRow()
	if err != nil {
		return &Board{Columns: DefaultColumns}, nil
	}
	var value string
	if err := row.Scan(&value); err != nil {
		return &Board{Columns: DefaultColumns}, nil
	}
	return parseBoard(value)
}

// writeBoard inserts a new board row. Previous rows are preserved
// as history.
func (t *Tasks) writeBoard(ctx context.Context, b *Board, author string) error {
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encoding board: %w", err)
	}
	now := time.Now().UnixMilli()
	_, err = t.db.Query(
		`INSERT INTO board (value, author, created_at) VALUES (?, ?, ?)`,
		string(data), author, now,
	).WithContext(ctx).Execute()
	return err
}

// parseBoard parses a board JSON value. Returns defaults on any error.
func parseBoard(value string) (*Board, error) {
	var b Board
	if err := json.Unmarshal([]byte(value), &b); err != nil {
		return &Board{Columns: DefaultColumns}, nil
	}
	if len(b.Columns) == 0 {
		return &Board{Columns: DefaultColumns}, nil
	}
	return &b, nil
}
