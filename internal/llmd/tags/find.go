package tags

import (
	"context"
	"fmt"
)

// FindOptions configures the Find operation.
type FindOptions struct {
	PathPrefix string // limit results to paths with this prefix
}

// Find returns all document paths that have the specified tag.
func (t *Tags) Find(ctx context.Context, tagName string, opts ...FindOptions) ([]string, error) {
	var opt FindOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query string
	var args []any

	if opt.PathPrefix != "" {
		query = `
			SELECT DISTINCT path
			FROM entities
			WHERE namespace = ? AND json_extract(value, '$.tag') = ?
			  AND path LIKE ? AND deleted_at IS NULL
			ORDER BY path
		`
		args = []any{namespace, tagName, opt.PathPrefix + "%"}
	} else {
		query = `
			SELECT DISTINCT path
			FROM entities
			WHERE namespace = ? AND json_extract(value, '$.tag') = ?
			  AND deleted_at IS NULL
			ORDER BY path
		`
		args = []any{namespace, tagName}
	}

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("finding documents by tag: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scanning path: %w", err)
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}
