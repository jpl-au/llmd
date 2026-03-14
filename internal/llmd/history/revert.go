package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/hash"
	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/internal/llmd/meta"
	"github.com/jpl-au/llmd/pkg/model/document"
)

// Revert restores a previous version as a new version. The hash
// check, version increment, and insert run in a single transaction
// so concurrent reverts cannot produce duplicate version numbers.
func (h *History) Revert(ctx context.Context, path string, version int, opts RevertOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Get the content of the version to revert to
	content, err := h.content(ctx, path, version)
	if err != nil {
		return nil, fmt.Errorf("getting version %d: %w", version, err)
	}

	result, err := h.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		return h.revertInTx(ctx, tx, path, content, version, opts)
	}).WithContext(ctx).Write()
	if err != nil {
		return nil, err
	}

	return result.Value.(*document.Document), nil
}

// revertInTx inserts the reverted content as a new version within
// an existing transaction.
func (h *History) revertInTx(ctx context.Context, tx *sql.Tx, path, content string, version int, opts RevertOptions) (*document.Document, error) {
	// Check if content is same as current latest (no-op)
	var latest string
	err := tx.QueryRowContext(ctx, `
		SELECT hash FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).Scan(&latest)

	s := hash.XXH3(content)
	if err == nil && latest == s {
		return h.latestInTx(ctx, tx, path)
	}

	// Get next version number
	var max int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM content
		WHERE namespace = ? AND path = ?
	`, namespace, path).Scan(&max)
	if err != nil {
		return nil, fmt.Errorf("getting max version: %w", err)
	}
	next := max + 1

	// Compute metadata
	m := meta.Compute(content)
	meta, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshaling meta: %w", err)
	}

	// Generate message if not provided
	message := opts.Message
	if message == "" {
		message = fmt.Sprintf("Reverted to version %d", version)
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO content (
			key, namespace, path, content, version, hash,
			author, message, source, mime, meta, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, path, content, next, s,
		opts.Author, message, opts.Source, mime, string(meta), now)
	if err != nil {
		return nil, fmt.Errorf("inserting reverted version: %w", err)
	}

	return &document.Document{
		Key:       k,
		Namespace: namespace,
		Path:      path,
		Content:   content,
		Version:   next,
		Hash:      s,
		Author:    opts.Author,
		Message:   message,
		Source:    opts.Source,
		MIME:      mime,
		Meta:      m,
		CreatedAt: now,
	}, nil
}

// content retrieves the content of a specific version.
func (h *History) content(ctx context.Context, path string, version int) (string, error) {
	var content string
	row, err := h.db.Query(`
		SELECT content FROM content
		WHERE namespace = ? AND path = ? AND version = ?
	`, namespace, path, version).WithContext(ctx).ReadRow()
	if err != nil {
		return "", err
	}
	if err := row.Scan(&content); err != nil {
		if err == sql.ErrNoRows {
			return "", ErrVersionInvalid
		}
		return "", err
	}
	return content, nil
}

// latestInTx reads the latest version of a document within a transaction.
func (h *History) latestInTx(ctx context.Context, tx *sql.Tx, path string) (*document.Document, error) {
	var doc document.Document
	var meta, message, mime sql.NullString
	var deletedAt sql.NullInt64

	err := tx.QueryRowContext(ctx, `
		SELECT id, key, namespace, path, content, version, hash,
		       author, message, source, mime, meta, created_at, deleted_at
		FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).Scan(
		&doc.ID, &doc.Key, &doc.Namespace, &doc.Path, &doc.Content,
		&doc.Version, &doc.Hash, &doc.Author, &message, &doc.Source,
		&mime, &meta, &doc.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if message.Valid {
		doc.Message = message.String
	}
	if mime.Valid {
		doc.MIME = mime.String
	}
	if meta.Valid && meta.String != "" {
		var m document.Meta
		if err := json.Unmarshal([]byte(meta.String), &m); err == nil {
			doc.Meta = &m
		}
	}

	return &doc, nil
}
