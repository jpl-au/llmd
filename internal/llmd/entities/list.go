package entities

import (
	"context"
	"database/sql"

	"github.com/jpl-au/llmd/pkg/model/entity"
)

// List returns entities in a namespace, optionally filtered by relation.
// Returns the latest state for each unique namespace+relation combination.
func (e *Entities) List(ctx context.Context, namespace string, opts ListOptions) ([]entity.Entity, error) {
	query := `
		SELECT id, key, namespace, relation, value, author, source, created_at, deleted_at
		FROM entities
		WHERE namespace = ? AND deleted_at IS NULL
	`
	args := []any{namespace}

	if opts.Relation != "" {
		query += ` AND relation = ?`
		args = append(args, opts.Relation)
	}

	query += ` ORDER BY created_at DESC`

	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := e.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []entity.Entity
	for rows.Next() {
		var ent entity.Entity
		var relation sql.NullString
		var deletedAt sql.NullInt64

		err := rows.Scan(
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
		if err != nil {
			return nil, err
		}

		if relation.Valid {
			ent.Relation = relation.String
		}
		if deletedAt.Valid {
			ent.DeletedAt = &deletedAt.Int64
		}

		entities = append(entities, ent)
	}

	return entities, rows.Err()
}
