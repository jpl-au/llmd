package entities

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jpl-au/llmd/internal/clock"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/entity"
)

// Write creates a new entity. Entities are insert-only; state changes create new rows.
func (e *Entities) Write(ctx context.Context, namespace, value string, opts WriteOptions) (*entity.Entity, error) {
	now := clock.Now()
	k := key.Generate()

	var relation sql.NullString
	if opts.Relation != "" {
		relation = sql.NullString{String: opts.Relation, Valid: true}
	}

	_, err := e.db.Query(`
		INSERT INTO entities (key, namespace, relation, value, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, relation, value, opts.Author, opts.Source, now).WithContext(ctx).Exec()
	if err != nil {
		return nil, fmt.Errorf("inserting entity: %w", err)
	}

	return &entity.Entity{
		Key:       k,
		Namespace: namespace,
		Relation:  opts.Relation,
		Value:     value,
		Author:    opts.Author,
		Source:    opts.Source,
		CreatedAt: now,
	}, nil
}
