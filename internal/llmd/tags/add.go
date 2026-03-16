package tags

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/tag"
)

// Add adds a tag to a document.
// value can be a document path or key.
// Returns ErrExists if the tag already exists on the document.
func (t *Tags) Add(ctx context.Context, value, name string, opts Options) (*tag.Tag, error) {
	if err := Validate(name); err != nil {
		return nil, err
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Resolve to get actual document path
	doc, err := t.docs.Resolve(ctx, value)
	if err != nil {
		return nil, err
	}
	relation := doc.Path

	// Check if tag already exists (latest non-deleted)
	existing, err := t.find(ctx, relation, name)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if existing != nil {
		return existing, ErrExists
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	tagValue := tag.Value{Tag: name}
	data, err := json.Marshal(tagValue)
	if err != nil {
		return nil, fmt.Errorf("marshaling tag: %w", err)
	}

	_, err = t.db.Query(`
		INSERT INTO entities (key, namespace, relation, value, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, relation, string(data), opts.Author, opts.Source, now).WithContext(ctx).Execute()

	if err != nil {
		return nil, fmt.Errorf("inserting tag: %w", err)
	}

	result := &tag.Tag{
		Key:      k,
		Relation: relation,
		Value:    tagValue,
		Origin: core.Origin{
			Author: opts.Author,
			Source: opts.Source,
		},
		CreatedAt: now,
	}

	if t.bus != nil {
		if err := t.bus.Emit(ctx, events.Event{
			Type:      events.TagAdded,
			Path:      relation,
			Key:       k,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"tag": name},
		}); err != nil {
			return nil, fmt.Errorf("emitting event: %w", err)
		}
	}

	return result, nil
}

// find retrieves a specific tag if it exists and is not deleted.
func (t *Tags) find(ctx context.Context, relation, name string) (*tag.Tag, error) {
	var tg tag.Tag
	var valueStr string
	var deletedAt sql.NullInt64

	row, err := t.db.Query(`
		SELECT key, relation, value, author, source, created_at, deleted_at
		FROM entities
		WHERE namespace = ? AND relation = ? AND json_extract(value, '$.tag') = ?
		AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT 1
	`, namespace, relation, name).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	if err := row.Scan(&tg.Key, &tg.Relation, &valueStr, &tg.Author, &tg.Source, &tg.CreatedAt, &deletedAt); err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		return nil, sql.ErrNoRows
	}

	if err := json.Unmarshal([]byte(valueStr), &tg.Value); err != nil {
		return nil, err
	}

	return &tg, nil
}
