package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/tag"
)

// List returns all tags for a document.
// value can be a document path or key.
func (t *Tags) List(ctx context.Context, value string, opts ...Options) ([]tag.Tag, error) {
	relation, err := t.resolvePath(ctx, value)
	if err != nil {
		return nil, err
	}

	rows, err := t.db.Query(`
		SELECT key, relation, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND relation = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, namespace, relation).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()

	return scanTags(rows)
}

// ListAll returns all unique tags across the store with document counts.
func (t *Tags) ListAll(ctx context.Context) ([]tag.Info, error) {
	rows, err := t.db.Query(`
		SELECT json_extract(value, '$.tag') as name, COUNT(DISTINCT relation) as count
		FROM entities
		WHERE namespace = ? AND deleted_at IS NULL
		GROUP BY name
		ORDER BY name
	`, namespace).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("querying all tags: %w", err)
	}
	defer rows.Close()

	var tags []tag.Info
	for rows.Next() {
		var info tag.Info
		if err := rows.Scan(&info.Name, &info.Count); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, info)
	}

	return tags, rows.Err()
}

func scanTags(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]tag.Tag, error) {
	var tags []tag.Tag

	for rows.Next() {
		var tg tag.Tag
		var valueStr string

		if err := rows.Scan(&tg.Key, &tg.Relation, &valueStr, &tg.Author, &tg.Source, &tg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}

		if err := json.Unmarshal([]byte(valueStr), &tg.Value); err != nil {
			return nil, fmt.Errorf("unmarshaling tag: %w", err)
		}

		tags = append(tags, tg)
	}

	return tags, rows.Err()
}
