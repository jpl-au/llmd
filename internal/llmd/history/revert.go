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

// Revert restores a previous version as a new version.
// The old version's content becomes the new latest version.
func (h *History) Revert(ctx context.Context, path string, version int, opts RevertOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Get the content of the version to revert to
	content, err := h.content(ctx, path, version)
	if err != nil {
		return nil, fmt.Errorf("getting version %d: %w", version, err)
	}

	// Get current max version
	var max int
	maxRow, err := h.db.Query(`
		SELECT COALESCE(MAX(version), 0) FROM content
		WHERE namespace = ? AND path = ?
	`, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("getting max version: %w", err)
	}
	if err := maxRow.Scan(&max); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting max version: %w", err)
	}
	next := max + 1

	// Check if content is same as current latest (no-op)
	var latest string
	hashRow, err := h.db.Query(`
		SELECT hash FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("getting latest hash: %w", err)
	}
	err = hashRow.Scan(&latest)

	s := hash.XXH3(content)
	if err == nil && latest == s {
		// Content unchanged, return existing
		return h.latest(ctx, path)
	}

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

	// Generate key and insert new version
	now := time.Now().UnixMilli()
	k := key.Generate()

	_, err = h.db.Query(`
		INSERT INTO content (
			key, namespace, path, content, version, hash,
			author, message, source, mime, meta, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, path, content, next, s,
		opts.Author, message, opts.Source, mime, string(meta), now).WithContext(ctx).Execute()
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

// latest reads the latest version of a document.
func (h *History) latest(ctx context.Context, path string) (*document.Document, error) {
	var doc document.Document
	var meta, message, mime sql.NullString
	var deletedAt sql.NullInt64

	row, err := h.db.Query(`
		SELECT id, key, namespace, path, content, version, hash,
		       author, message, source, mime, meta, created_at, deleted_at
		FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}

	err = row.Scan(
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
