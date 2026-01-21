package links

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/link"
)

// List returns links for a document.
// value can be a document path or 9-char key.
// opts.Direction controls which links to return (Outgoing, Incoming, or Both).
func (l *Links) List(ctx context.Context, value string, opts ...Options) ([]link.Link, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Resolve document
	doc, err := l.docs.Resolve(ctx, value)
	if err != nil {
		return nil, err
	}
	path := doc.Path

	// Default to Outgoing if not specified
	dir := opt.Direction
	if dir == 0 {
		dir = Outgoing
	}

	var links []link.Link

	// Get outgoing links (from this document)
	if dir == Outgoing || dir == Both {
		outgoing, err := l.outgoing(ctx, path)
		if err != nil {
			return nil, err
		}
		links = append(links, outgoing...)
	}

	// Get incoming links (to this document)
	if dir == Incoming || dir == Both {
		incoming, err := l.incoming(ctx, path)
		if err != nil {
			return nil, err
		}
		links = append(links, incoming...)
	}

	return links, nil
}

// outgoing returns links where this document is the source.
func (l *Links) outgoing(ctx context.Context, path string) ([]link.Link, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, key, path, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, namespace, path)
	if err != nil {
		return nil, fmt.Errorf("querying outgoing links: %w", err)
	}
	defer rows.Close()

	return scan(rows)
}

// incoming returns links where this document is the target.
func (l *Links) incoming(ctx context.Context, path string) ([]link.Link, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, key, path, value, author, source, created_at
		FROM entities
		WHERE namespace = ? AND deleted_at IS NULL
		  AND json_extract(value, '$.to') = ?
		ORDER BY created_at DESC
	`, namespace, path)
	if err != nil {
		return nil, fmt.Errorf("querying incoming links: %w", err)
	}
	defer rows.Close()

	return scan(rows)
}

func scan(rows interface{ Next() bool; Scan(...any) error; Err() error }) ([]link.Link, error) {
	var links []link.Link

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

		links = append(links, lk)
	}

	return links, rows.Err()
}
