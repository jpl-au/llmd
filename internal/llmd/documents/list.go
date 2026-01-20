package documents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// List returns documents matching the given options.
// Returns document.Info (without content) for efficiency.
func (d *Documents) List(ctx context.Context, opts ...ListOptions) ([]document.Info, error) {
	var opt ListOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT key, path, version, author, message, source, mime, meta, created_at, deleted_at
		FROM content
		WHERE namespace = ?
	`)
	args = append(args, namespace)

	if opt.Prefix != "" {
		query.WriteString(" AND path LIKE ?")
		args = append(args, opt.Prefix+"%")
	}

	if !opt.IncludeDeleted {
		query.WriteString(" AND deleted_at IS NULL")
	}

	// Only get latest version of each path
	query.WriteString(`
		AND version = (
			SELECT MAX(version) FROM content c2
			WHERE c2.namespace = content.namespace AND c2.path = content.path
		)
		ORDER BY path
	`)

	if opt.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opt.Limit)
	}

	rows, err := d.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("querying documents: %w", err)
	}
	defer rows.Close()

	var results []document.Info
	for rows.Next() {
		var info document.Info
		var message, mime, meta *string
		var deletedAt *int64

		err := rows.Scan(
			&info.Key, &info.Path, &info.Version, &info.Author,
			&message, &info.Source, &mime, &meta, &info.CreatedAt, &deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if message != nil {
			info.Message = *message
		}
		if mime != nil {
			info.MIME = *mime
		}
		if deletedAt != nil {
			info.DeletedAt = deletedAt
		}
		if meta != nil && *meta != "" {
			var m document.Meta
			if err := json.Unmarshal([]byte(*meta), &m); err == nil {
				info.Meta = &m
			}
		}

		results = append(results, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}
