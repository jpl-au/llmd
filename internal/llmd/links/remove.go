package links

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/pkg/model/link"
)

// Remove removes a link between two documents.
// If opts.Label is empty, removes all links from→to.
// from and to can be document paths or keys.
func (l *Links) Remove(ctx context.Context, from, to string, opts Options) error {
	// Resolve both documents
	fromDoc, err := l.docs.Resolve(ctx, from)
	if err != nil {
		return fmt.Errorf("resolving from: %w", err)
	}
	toDoc, err := l.docs.Resolve(ctx, to)
	if err != nil {
		return fmt.Errorf("resolving to: %w", err)
	}

	relation := fromDoc.Path
	toPath := toDoc.Path

	// Find matching links
	rows, err := l.db.QueryContext(ctx, `
		SELECT key, value
		FROM entities
		WHERE namespace = ? AND relation = ? AND deleted_at IS NULL
	`, namespace, relation)
	if err != nil {
		return fmt.Errorf("querying links: %w", err)
	}
	defer rows.Close()

	var keysToDelete []string
	for rows.Next() {
		var k, valueStr string
		if err := rows.Scan(&k, &valueStr); err != nil {
			return fmt.Errorf("scanning link: %w", err)
		}

		var v link.Value
		if err := json.Unmarshal([]byte(valueStr), &v); err != nil {
			return fmt.Errorf("unmarshaling link: %w", err)
		}

		// Match by target, and optionally by label
		if v.To == toPath {
			if opts.Label == "" || v.Label == opts.Label {
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

	// Soft delete the matching links in a single transaction.
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	for _, k := range keysToDelete {
		_, err := tx.ExecContext(ctx, `
			UPDATE entities SET deleted_at = ? WHERE key = ?
		`, now, k)
		if err != nil {
			return fmt.Errorf("deleting link: %w", err)
		}
	}

	return tx.Commit()
}
