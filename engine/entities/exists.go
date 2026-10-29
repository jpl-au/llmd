package entities

import (
	"context"
	"database/sql"

	"github.com/jpl-au/llmd/pkg/model/entity"
)

// Exists checks if an entity exists by key.
func (e *Entities) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	row, err := e.db.Query(`
		SELECT EXISTS(
			SELECT 1 FROM entities WHERE key = ? AND deleted_at IS NULL
		)
	`, key).WithContext(ctx).ReadRow()
	if err != nil {
		return false, err
	}
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// ExistsInNamespace checks if an entity exists with the given namespace and relation.
func (e *Entities) ExistsInNamespace(ctx context.Context, namespace, relation string) (bool, error) {
	var exists bool
	var row *sql.Row
	var err error

	if relation == "" {
		row, err = e.db.Query(`
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation IS NULL AND deleted_at IS NULL
			)
		`, namespace).WithContext(ctx).ReadRow()
	} else {
		row, err = e.db.Query(`
			SELECT EXISTS(
				SELECT 1 FROM entities
				WHERE namespace = ? AND relation = ? AND deleted_at IS NULL
			)
		`, namespace, relation).WithContext(ctx).ReadRow()
	}
	if err != nil {
		return false, err
	}
	if err := row.Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

// FindByValue finds an entity by namespace, relation, and a JSON value match.
// This is useful for checking if a specific tag or link already exists.
func (e *Entities) FindByValue(ctx context.Context, namespace, relation, jsonPath, jsonValue string) (*entity.Entity, error) {
	var query string
	var args []any

	if relation == "" {
		query = `
			SELECT id, key, namespace, relation, value, author, source, created_at, deleted_at
			FROM entities
			WHERE namespace = ? AND relation IS NULL
			AND json_extract(value, ?) = ? AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT 1
		`
		args = []any{namespace, jsonPath, jsonValue}
	} else {
		query = `
			SELECT id, key, namespace, relation, value, author, source, created_at, deleted_at
			FROM entities
			WHERE namespace = ? AND relation = ?
			AND json_extract(value, ?) = ? AND deleted_at IS NULL
			ORDER BY created_at DESC LIMIT 1
		`
		args = []any{namespace, relation, jsonPath, jsonValue}
	}

	row, err := e.db.Query(query, args...).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}

	var ent entity.Entity
	var rel sql.NullString
	var deletedAt sql.NullInt64

	err = row.Scan(
		&ent.ID,
		&ent.Key,
		&ent.Namespace,
		&rel,
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

	if rel.Valid {
		ent.Relation = rel.String
	}
	if deletedAt.Valid {
		ent.DeletedAt = &deletedAt.Int64
	}

	return &ent, nil
}
