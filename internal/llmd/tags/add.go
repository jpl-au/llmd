package tags

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/tag"
)

// Add adds a tag to a document.
// pathOrKey can be a document path or 9-char key.
// If the tag already exists on the document, returns the existing tag.
func (t *Tags) Add(ctx context.Context, pathOrKey, tagName string, opts Options) (*tag.Tag, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, pathOrKey)
	if err != nil {
		return nil, err
	}
	path := doc.Path

	// Check if tag already exists (latest non-deleted)
	existing, err := t.get(ctx, path, tagName)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	value, err := json.Marshal(map[string]string{"tag": tagName})
	if err != nil {
		return nil, fmt.Errorf("marshaling tag: %w", err)
	}

	result, err := t.db.ExecContext(ctx, `
		INSERT INTO entities (key, namespace, path, value, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, path, string(value), opts.Author, opts.Source, now)

	if err != nil {
		return nil, fmt.Errorf("inserting tag: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting insert id: %w", err)
	}

	return &tag.Tag{
		ID:        id,
		Key:       k,
		Path:      path,
		Tag:       tagName,
		Author:    opts.Author,
		Source:    opts.Source,
		CreatedAt: now,
	}, nil
}

// get retrieves a specific tag if it exists and is not deleted.
func (t *Tags) get(ctx context.Context, path, tagName string) (*tag.Tag, error) {
	var tg tag.Tag
	var value string
	var deletedAt sql.NullInt64

	err := t.db.QueryRowContext(ctx, `
		SELECT id, key, path, value, author, source, created_at, deleted_at
		FROM entities
		WHERE namespace = ? AND path = ? AND json_extract(value, '$.tag') = ?
		ORDER BY created_at DESC LIMIT 1
	`, namespace, path, tagName).Scan(&tg.ID, &tg.Key, &tg.Path, &value, &tg.Author, &tg.Source, &tg.CreatedAt, &deletedAt)

	if err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		return nil, sql.ErrNoRows
	}

	var v map[string]string
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return nil, err
	}
	tg.Tag = v["tag"]

	return &tg, nil
}
