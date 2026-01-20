package links

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/link"
)

// Add creates a link between two documents.
// fromPathOrKey and toPathOrKey can be document paths or 9-char keys.
func (l *Links) Add(ctx context.Context, fromPathOrKey, toPathOrKey string, opts Options) (*link.Link, error) {
	// Resolve both documents
	fromDoc, err := l.docs.Resolve(ctx, fromPathOrKey)
	if err != nil {
		return nil, fmt.Errorf("resolving from: %w", err)
	}
	toDoc, err := l.docs.Resolve(ctx, toPathOrKey)
	if err != nil {
		return nil, fmt.Errorf("resolving to: %w", err)
	}

	fromPath := fromDoc.Path
	toPath := toDoc.Path

	// Check for self-link
	if fromPath == toPath {
		return nil, ErrSelfLink
	}

	// Check if link already exists
	existing, err := l.findLink(ctx, fromPath, toPath, opts.Label)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// Create link value
	value, err := json.Marshal(map[string]string{
		"to":    toPath,
		"label": opts.Label,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling link: %w", err)
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	result, err := l.db.ExecContext(ctx, `
		INSERT INTO entities (key, namespace, path, value, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, fromPath, string(value), opts.Author, opts.Source, now)
	if err != nil {
		return nil, fmt.Errorf("inserting link: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting insert id: %w", err)
	}

	return &link.Link{
		ID:        id,
		Key:       k,
		From:      fromPath,
		To:        toPath,
		Label:     opts.Label,
		Author:    opts.Author,
		Source:    opts.Source,
		CreatedAt: now,
	}, nil
}

// findLink checks if a link already exists.
func (l *Links) findLink(ctx context.Context, from, to, label string) (*link.Link, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, key, path, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
	`, namespace, from)
	if err != nil {
		return nil, fmt.Errorf("querying links: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var lk link.Link
		var value string

		if err := rows.Scan(&lk.ID, &lk.Key, &lk.From, &value, &lk.Author, &lk.Source, &lk.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}

		var v map[string]string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return nil, fmt.Errorf("unmarshaling link: %w", err)
		}

		lk.To = v["to"]
		lk.Label = v["label"]

		if lk.To == to && lk.Label == label {
			return &lk, nil
		}
	}

	return nil, rows.Err()
}
