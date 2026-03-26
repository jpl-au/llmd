// columns.go manages board columns and pipeline configuration.

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// StepConfig configures automatic agent spawning for a column.
type StepConfig struct {
	Agent     string `json:"agent"`
	Role      string `json:"role"`
	OnSuccess string `json:"on_success,omitempty"`
	OnFailure string `json:"on_failure,omitempty"`
}

// Board holds the column list and optional pipeline configuration.
type Board struct {
	Columns  []string              `json:"columns"`
	Pipeline map[string]StepConfig `json:"pipeline,omitempty"`
}

// Columns returns the board columns in order.
func (t *Tasks) Columns(ctx context.Context) ([]string, error) {
	b, err := t.readBoard(ctx)
	if err != nil {
		return nil, err
	}
	return b.Columns, nil
}

// Board returns the full board including pipeline configuration.
func (t *Tasks) Board(ctx context.Context) (*Board, error) {
	return t.readBoard(ctx)
}

// Step returns the pipeline configuration for a column, or nil if
// the column has no pipeline step configured.
func (t *Tasks) Step(ctx context.Context, column string) (*StepConfig, error) {
	b, err := t.readBoard(ctx)
	if err != nil {
		return nil, err
	}
	if b.Pipeline == nil {
		return nil, nil
	}
	cfg, ok := b.Pipeline[column]
	if !ok {
		return nil, nil
	}
	return &cfg, nil
}

// SetStep configures a pipeline step for a column.
func (t *Tasks) SetStep(ctx context.Context, column string, cfg StepConfig, author string) error {
	b, err := t.readBoard(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(b.Columns, column) {
		return fmt.Errorf("%w: %s", ErrColNotFound, column)
	}

	if b.Pipeline == nil {
		b.Pipeline = make(map[string]StepConfig)
	}
	b.Pipeline[column] = cfg

	return t.writeBoard(ctx, b, author)
}

// UnsetStep removes pipeline configuration from a column.
func (t *Tasks) UnsetStep(ctx context.Context, column, author string) error {
	b, err := t.readBoard(ctx)
	if err != nil {
		return err
	}

	if b.Pipeline == nil {
		return nil
	}
	delete(b.Pipeline, column)

	return t.writeBoard(ctx, b, author)
}

// AddColumn adds a new column. If after is empty, appends to the end.
func (t *Tasks) AddColumn(ctx context.Context, name, after, author string) error {
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

	// Remove pipeline config for the deleted column.
	if b.Pipeline != nil {
		delete(b.Pipeline, name)
	}

	return t.writeBoard(ctx, b, author)
}

// MoveColumn reorders a column to be after another column.
func (t *Tasks) MoveColumn(ctx context.Context, name, after, author string) error {
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

// readBoard reads the full board from the entity store. Handles
// both the old format (columns-only array) and the new format
// (object with columns and pipeline).
func (t *Tasks) readBoard(ctx context.Context) (*Board, error) {
	exists, err := t.entities.ExistsInNamespace(ctx, boardNamespace, "")
	if err != nil {
		return nil, err
	}
	if !exists {
		return &Board{Columns: DefaultColumns}, nil
	}

	ents, err := t.entities.List(ctx, boardNamespace, entities.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(ents) == 0 {
		return &Board{Columns: DefaultColumns}, nil
	}

	return parseBoard(ents[0].Value)
}

// writeBoard serialises the full board and writes it to the entity
// store. Soft-deletes the old entity and creates a new one.
func (t *Tasks) writeBoard(ctx context.Context, b *Board, author string) error {
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

	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encoding board: %w", err)
	}

	_, err = t.entities.Write(ctx, boardNamespace, string(data), entities.WriteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
	return err
}

// parseBoard parses a board entity value. Handles both formats:
// old: {"columns":["a","b"]} (no pipeline key)
// new: {"columns":["a","b"],"pipeline":{"a":{...}}}
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
