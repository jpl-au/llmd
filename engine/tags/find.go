package tags

import (
	"context"
	"fmt"
)

// FindOptions configures the Find operation.
type FindOptions struct {
	RelationPrefix string // limit results to relations with this prefix
}

// Find returns all document paths (relations) that have the specified tag.
func (t *Tags) Find(ctx context.Context, name string, opts ...FindOptions) ([]string, error) {
	if err := Validate(name); err != nil {
		return nil, err
	}

	var opt FindOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query string
	var args []any

	if opt.RelationPrefix != "" {
		query = `
			SELECT DISTINCT relation
			FROM entities
			WHERE namespace = ? AND json_extract(value, '$.tag') = ?
			  AND relation LIKE ? AND deleted_at IS NULL
			ORDER BY relation
		`
		args = []any{namespace, name, opt.RelationPrefix + "%"}
	} else {
		query = `
			SELECT DISTINCT relation
			FROM entities
			WHERE namespace = ? AND json_extract(value, '$.tag') = ?
			  AND deleted_at IS NULL
			ORDER BY relation
		`
		args = []any{namespace, name}
	}

	rows, err := t.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("finding documents by tag: %w", err)
	}
	defer rows.Close()

	var relations []string
	for rows.Next() {
		var relation string
		if err := rows.Scan(&relation); err != nil {
			return nil, fmt.Errorf("scanning relation: %w", err)
		}
		relations = append(relations, relation)
	}

	return relations, rows.Err()
}
