package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// Read retrieves a document by path.
// Returns ErrNotFound if the document doesn't exist.
// Returns ErrDeleted if the document is soft-deleted.
func (d *Documents) Read(ctx context.Context, path string, opts ...ReadOptions) (*document.Document, error) {
	var opt ReadOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query string
	var args []any

	if opt.Version != nil {
		// Specific version
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE namespace = ? AND path = ? AND version = ?
		`
		args = []any{namespace, path, *opt.Version}
	} else {
		// Latest version
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE namespace = ? AND path = ?
			ORDER BY version DESC LIMIT 1
		`
		args = []any{namespace, path}
	}

	row, err := d.db.Query(query, args...).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("querying document: %w", err)
	}
	doc, err := d.scan(row)
	if doc != nil {
		doc.Resolved = document.ResolvedPath
	}
	return doc, d.explain(ctx, err, path, opt.Version)
}

// ReadByKey retrieves a document by its stable key. Without a version
// option the latest version is returned; with a version the specific
// version is returned.
func (d *Documents) ReadByKey(ctx context.Context, key string, opts ...ReadOptions) (*document.Document, error) {
	var opt ReadOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query string
	var args []any

	if opt.Version != nil {
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE key = ? AND version = ?
		`
		args = []any{key, *opt.Version}
	} else {
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE key = ?
			ORDER BY version DESC LIMIT 1
		`
		args = []any{key}
	}

	row, err := d.db.Query(query, args...).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("querying document: %w", err)
	}
	doc, err := d.scan(row)
	if doc != nil {
		doc.Resolved = document.ResolvedKey
	}
	return doc, d.explain(ctx, err, key, nil)
}

// readInTx reads a document within an existing transaction.
func (d *Documents) readInTx(ctx context.Context, tx *sql.Tx, path string, opts ...ReadOptions) (*document.Document, error) {
	var opt ReadOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	var query string
	var args []any

	if opt.Version != nil {
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE namespace = ? AND path = ? AND version = ?
		`
		args = []any{namespace, path, *opt.Version}
	} else {
		query = `
			SELECT id, key, namespace, path, content, version, hash,
			       author, message, source, mime, meta, created_at, deleted_at
			FROM content
			WHERE namespace = ? AND path = ?
			ORDER BY version DESC LIMIT 1
		`
		args = []any{namespace, path}
	}

	doc, err := d.scan(tx.QueryRowContext(ctx, query, args...))
	if doc != nil {
		doc.Resolved = document.ResolvedPath
	}
	switch err {
	case ErrNotFound:
		return doc, NotFound(path)
	case ErrDeleted:
		return doc, Deleted(path)
	}
	return doc, err
}

// scan scans a row into a Document.
// explain names the document in the bare ErrNotFound or ErrDeleted that
// scan returns. A missing version of a live document says which version
// is missing; any other miss reports the document's state.
func (d *Documents) explain(ctx context.Context, err error, name string, version *int) error {
	switch err {
	case ErrDeleted:
		return Deleted(name)
	case ErrNotFound:
		if version == nil {
			return NotFound(name)
		}
		if cerr := d.Check(ctx, name); cerr != nil {
			return cerr
		}
		return NoVersion(ErrNotFound, name, *version)
	}
	return err
}

func (d *Documents) scan(row *sql.Row) (*document.Document, error) {
	var doc document.Document
	var meta sql.NullString
	var message sql.NullString
	var mime sql.NullString
	var deletedAt sql.NullInt64

	err := row.Scan(
		&doc.ID, &doc.Key, &doc.Namespace, &doc.Path, &doc.Content,
		&doc.Version, &doc.Hash, &doc.Author, &message, &doc.Source,
		&mime, &meta, &doc.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning document: %w", err)
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

	// A soft-deleted document is returned in full alongside ErrDeleted
	// so callers can still inspect it (for example to decide whether
	// to restore).
	if deletedAt.Valid {
		doc.DeletedAt = &deletedAt.Int64
		return &doc, ErrDeleted
	}

	return &doc, nil
}
