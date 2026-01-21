package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/tag"
)

// List returns all tags for a document.
// value can be a document path or 9-char key.
func (t *Tags) List(ctx context.Context, value string, opts ...Options) ([]tag.Tag, error) {
	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, value)
	if err != nil {
		return nil, err
	}
	path := doc.Path

	rows, err := t.db.QueryContext(ctx, `
		SELECT id, key, path, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, namespace, path)
	if err != nil {
		return nil, fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()

	return scan(rows)
}

// ListAll returns all unique tags across the store with document counts.
func (t *Tags) ListAll(ctx context.Context) ([]tag.Info, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT json_extract(value, '$.tag') as name, COUNT(DISTINCT path) as count
		FROM entities
		WHERE namespace = ? AND deleted_at IS NULL
		GROUP BY name
		ORDER BY name
	`, namespace)
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

func scan(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]tag.Tag, error) {
	var tags []tag.Tag

	for rows.Next() {
		var tg tag.Tag
		var value string

		if err := rows.Scan(&tg.ID, &tg.Key, &tg.Path, &value, &tg.Author, &tg.Source, &tg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}

		var v map[string]string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return nil, fmt.Errorf("unmarshaling tag: %w", err)
		}
		tg.Tag = v["tag"]

		tags = append(tags, tg)
	}

	return tags, rows.Err()
}
