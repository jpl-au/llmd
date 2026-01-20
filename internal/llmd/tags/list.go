package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/tag"
)

// List returns all tags for a document.
// pathOrKey can be a document path or 9-char key.
func (t *Tags) List(ctx context.Context, pathOrKey string, opts ...Options) ([]tag.Tag, error) {
	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, pathOrKey)
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

	return scanTags(rows)
}

// ListAll returns all unique tag names across the store.
func (t *Tags) ListAll(ctx context.Context) ([]string, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT DISTINCT json_extract(value, '$.tag') as tag_name
		FROM entities
		WHERE namespace = ? AND deleted_at IS NULL
		ORDER BY tag_name
	`, namespace)
	if err != nil {
		return nil, fmt.Errorf("querying all tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, name)
	}

	return tags, rows.Err()
}

func scanTags(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]tag.Tag, error) {
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
