package links

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/internal/clock"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/link"
)

// Add creates a link between two documents.
// from and to can be document paths or keys.
func (l *Links) Add(ctx context.Context, from, to string, opts Options) (*link.Link, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	relation := l.resolvePath(ctx, from)
	if err := l.requireDocument(ctx, relation); err != nil {
		return nil, fmt.Errorf("resolving from: %w", err)
	}
	toPath := l.resolvePath(ctx, to)
	if err := l.requireDocument(ctx, toPath); err != nil {
		return nil, fmt.Errorf("resolving to: %w", err)
	}

	// Check for self-link
	if relation == toPath {
		return nil, ErrSelfLink
	}

	// Check if link already exists
	existing, err := l.find(ctx, relation, toPath, opts.Label)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, ErrExists
	}

	// Create link value
	linkValue := link.Value{
		To:    toPath,
		Label: opts.Label,
	}
	data, err := json.Marshal(linkValue)
	if err != nil {
		return nil, fmt.Errorf("marshaling link: %w", err)
	}

	now := clock.Now()

	k, err := key.Insert("entities.key", func(k string) error {
		_, err := l.db.Query(`
			INSERT INTO entities (key, namespace, relation, value, author, source, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, k, namespace, relation, string(data), opts.Author, opts.Source, now).WithContext(ctx).Exec()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("inserting link: %w", err)
	}

	result := &link.Link{
		Key:       k,
		Relation:  relation,
		Value:     linkValue,
		Author:    opts.Author,
		Source:    opts.Source,
		CreatedAt: now,
	}

	if l.bus != nil {
		if err := l.bus.Emit(ctx, events.Event{
			Type:      events.LinkCreated,
			Path:      relation,
			Key:       k,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"to": toPath, "label": opts.Label},
		}); err != nil {
			return nil, fmt.Errorf("emitting event: %w", err)
		}
	}

	return result, nil
}

// find checks if a link already exists.
func (l *Links) find(ctx context.Context, relation, to, label string) (*link.Link, error) {
	rows, err := l.db.Query(`
		SELECT key, relation, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND relation = ? AND deleted_at IS NULL
	`, namespace, relation).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("querying links: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var lk link.Link
		var valueStr string

		if err := rows.Scan(&lk.Key, &lk.Relation, &valueStr, &lk.Author, &lk.Source, &lk.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning link: %w", err)
		}

		if err := json.Unmarshal([]byte(valueStr), &lk.Value); err != nil {
			return nil, fmt.Errorf("unmarshaling link: %w", err)
		}

		if lk.Value.To == to && lk.Value.Label == label {
			return &lk, nil
		}
	}

	return nil, rows.Err()
}
