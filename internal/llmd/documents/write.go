package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/hash"
	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/internal/llmd/meta"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/document"
)

const namespace = "core:document"
const mime = "text/markdown"

// Write creates or updates a document at the given path. The hash
// check, version increment, and insert run in a single transaction
// so concurrent writes cannot produce duplicate version numbers.
func (d *Documents) Write(ctx context.Context, path, content string, opts WriteOptions) (*document.Document, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	doc, err := d.writeInTx(ctx, tx, path, content, opts)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	// Emit event after commit so subscribers see committed data.
	if d.bus != nil {
		if err := d.bus.Emit(ctx, events.Event{
			Type:      events.DocumentWritten,
			Path:      path,
			Key:       doc.Key,
			Version:   doc.Version,
			Author:    opts.Author,
			Timestamp: doc.CreatedAt,
		}); err != nil {
			return nil, fmt.Errorf("emitting event: %w", err)
		}
	}

	return doc, nil
}

// writeInTx writes a document within an existing transaction.
func (d *Documents) writeInTx(ctx context.Context, tx *sql.Tx, path, content string, opts WriteOptions) (*document.Document, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	s := hash.XXH3(content)

	// Check if content is unchanged from latest version
	var latest string
	err := tx.QueryRowContext(ctx, `
		SELECT hash FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).Scan(&latest)

	if err == nil && latest == s {
		// Content unchanged, return existing document
		return d.readInTx(ctx, tx, path)
	}

	// Get next version number
	var max int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM content
		WHERE namespace = ? AND path = ?
	`, namespace, path).Scan(&max)
	if err != nil {
		return nil, fmt.Errorf("getting version: %w", err)
	}
	next := max + 1

	// Compute metadata
	m := meta.Compute(content)
	meta, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshaling meta: %w", err)
	}

	// Generate key and insert
	k := key.Generate()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO content (
			key, namespace, path, content, version, hash,
			author, message, source, mime, meta, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, path, content, next, s,
		opts.Author, opts.Message, opts.Source, mime, string(meta), now)

	if err != nil {
		return nil, fmt.Errorf("inserting document: %w", err)
	}

	// Note: Events are emitted by caller after transaction commits

	return &document.Document{
		Key:       k,
		Namespace: namespace,
		Path:      path,
		Content:   content,
		Version:   next,
		Hash:      s,
		Author:    opts.Author,
		Message:   opts.Message,
		Source:    opts.Source,
		MIME:      mime,
		Meta:      m,
		CreatedAt: now,
	}, nil
}
