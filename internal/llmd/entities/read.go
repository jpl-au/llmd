package entities

import (
	"context"
	"database/sql"

	"github.com/jpl-au/llmd/pkg/model/entity"
)

// Read gets an entity by key.
func (e *Entities) Read(ctx context.Context, key string) (*entity.Entity, error) {
	row, err := e.db.Query(`
		SELECT id, key, namespace, relation, value, author, source, created_at, deleted_at
		FROM entities
		WHERE key = ? AND deleted_at IS NULL
	`, key).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}

	return e.scan(row)
}

func (e *Entities) scan(row *sql.Row) (*entity.Entity, error) {
	var ent entity.Entity
	var relation sql.NullString
	var deletedAt sql.NullInt64

	err := row.Scan(
		&ent.ID,
		&ent.Key,
		&ent.Namespace,
		&relation,
		&ent.Value,
		&ent.Author,
		&ent.Source,
		&ent.CreatedAt,
		&deletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if relation.Valid {
		ent.Relation = relation.String
	}
	if deletedAt.Valid {
		ent.DeletedAt = &deletedAt.Int64
	}

	return &ent, nil
}
