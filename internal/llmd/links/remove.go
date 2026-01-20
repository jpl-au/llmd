package links

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Remove removes a link between two documents.
// If opts.Label is empty, removes all links from→to.
// fromPathOrKey and toPathOrKey can be document paths or 9-char keys.
func (l *Links) Remove(ctx context.Context, fromPathOrKey, toPathOrKey string, opts Options) error {
	// Resolve both documents
	fromDoc, err := l.docs.Resolve(ctx, fromPathOrKey)
	if err != nil {
		return fmt.Errorf("resolving from: %w", err)
	}
	toDoc, err := l.docs.Resolve(ctx, toPathOrKey)
	if err != nil {
		return fmt.Errorf("resolving to: %w", err)
	}

	fromPath := fromDoc.Path
	toPath := toDoc.Path

	// Find matching links
	rows, err := l.db.QueryContext(ctx, `
		SELECT key, value
		FROM entities
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
	`, namespace, fromPath)
	if err != nil {
		return fmt.Errorf("querying links: %w", err)
	}
	defer rows.Close()

	var keysToDelete []string
	for rows.Next() {
		var k, value string
		if err := rows.Scan(&k, &value); err != nil {
			return fmt.Errorf("scanning link: %w", err)
		}

		var v map[string]string
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("unmarshaling link: %w", err)
		}

		// Match by target, and optionally by label
		if v["to"] == toPath {
			if opts.Label == "" || v["label"] == opts.Label {
				keysToDelete = append(keysToDelete, k)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(keysToDelete) == 0 {
		return ErrNotFound
	}

	// Soft delete the links
	now := time.Now().UnixMilli()
	for _, k := range keysToDelete {
		_, err := l.db.ExecContext(ctx, `
			UPDATE entities SET deleted_at = ? WHERE key = ?
		`, now, k)
		if err != nil {
			return fmt.Errorf("deleting link: %w", err)
		}
	}

	return nil
}
