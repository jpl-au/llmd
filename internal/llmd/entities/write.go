package entities

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/entity"
)

// Write creates a new entity. Entities are insert-only; state changes create new rows.
func (e *Entities) Write(ctx context.Context, namespace, value string, opts WriteOptions) (*entity.Entity, error) {
	now := time.Now().UnixMilli()
	k := key.Generate()

	var relation sql.NullString
	if opts.Relation != "" {
		relation = sql.NullString{String: opts.Relation, Valid: true}
	}

	_, err := e.db.ExecContext(ctx, `
		INSERT INTO entities (key, namespace, relation, value, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, relation, value, opts.Author, opts.Source, now)

	if err != nil {
		return nil, fmt.Errorf("inserting entity: %w", err)
	}

	return &entity.Entity{
		Key:       k,
		Namespace: namespace,
		Relation:  opts.Relation,
		Value:     value,
		Origin: core.Origin{
			Author: opts.Author,
			Source: opts.Source,
		},
		CreatedAt: now,
	}, nil
}
