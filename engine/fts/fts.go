// Package fts keeps the full-text index in step with the content table.
//
// content_fts is an external-content FTS5 table: it stores only the
// index and reads column values from content. Removing a row from such
// an index means replaying the exact values it was indexed with, and
// replaying a row that is not indexed corrupts the index. So the index
// follows one rule that can be read back from content at any time: each
// path with a live version has exactly one indexed row, its latest live
// version.
//
// Every change to content calls Live before the change and Sync after
// it, on the same transaction, so the index commits or rolls back with
// the change itself.
package fts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const namespace = "core:document"

// Row is the indexed form of a document version.
type Row struct {
	ID      int64
	Key     string
	Path    string
	Content string
}

// Live returns the row the index holds for path: its latest live
// version, or nil when the path has none.
func Live(ctx context.Context, tx *sql.Tx, path string) (*Row, error) {
	var r Row
	err := tx.QueryRowContext(ctx, `
		SELECT id, key, path, content FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).Scan(&r.ID, &r.Key, &r.Path, &r.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading indexed version of %s: %w", path, err)
	}
	return &r, nil
}

// Sync moves the index from before, the row Live returned before the
// change, to the row path holds now. A move passes the destination as
// path. Nothing is written when the indexed row did not change.
func Sync(ctx context.Context, tx *sql.Tx, before *Row, path string) error {
	after, err := Live(ctx, tx, path)
	if err != nil {
		return err
	}
	if before != nil && after != nil && *before == *after {
		return nil
	}
	if before != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO content_fts(content_fts, rowid, key, path, content)
			VALUES ('delete', ?, ?, ?, ?)
		`, before.ID, before.Key, before.Path, before.Content); err != nil {
			return fmt.Errorf("removing %s from the search index: %w", before.Path, err)
		}
	}
	if after != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO content_fts(rowid, key, path, content)
			VALUES (?, ?, ?, ?)
		`, after.ID, after.Key, after.Path, after.Content); err != nil {
			return fmt.Errorf("adding %s to the search index: %w", after.Path, err)
		}
	}
	return nil
}
