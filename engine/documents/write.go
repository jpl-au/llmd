package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/clock"

	"github.com/jpl-au/llmd/engine/fts"
	"github.com/jpl-au/llmd/engine/hash"
	"github.com/jpl-au/llmd/engine/key"
	"github.com/jpl-au/llmd/engine/meta"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/document"
)

const namespace = "core:document"
const mime = "text/markdown"

// Write creates or updates a document at the given path. The hash
// check, version increment, insert and search index update run in a
// single transaction so concurrent writes cannot produce duplicate
// version numbers and the index cannot fall behind the content.
func (d *Documents) Write(ctx context.Context, path, content string, opts WriteOptions) (*document.Document, error) {
	result, err := d.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		return d.writeInTx(ctx, tx, path, content, opts)
	}).WithContext(ctx).Exec()
	if err != nil {
		return nil, err
	}

	w := result.Value.(written)

	// Emit event after commit so subscribers see committed data. A
	// write that changed nothing is not an event; one that only brought
	// a deleted document back is a restore.
	if d.bus != nil && (w.changed || w.restored) {
		typ, at := events.DocumentWritten, w.doc.CreatedAt
		if !w.changed {
			typ, at = events.DocumentRestored, clock.Now()
		}
		d.bus.Emit(ctx, events.Event{
			Type:      typ,
			Path:      path,
			Key:       w.doc.Key,
			Version:   w.doc.Version,
			Author:    opts.Author,
			Timestamp: at,
		})
	}

	return w.doc, nil
}

// written is the result of writeInTx: the document as it now stands,
// whether the write added a version, and whether it brought a deleted
// document back without one.
type written struct {
	doc      *document.Document
	changed  bool
	restored bool
}

// writeInTx writes a document within an existing transaction and
// updates the search index on the same transaction.
func (d *Documents) writeInTx(ctx context.Context, tx *sql.Tx, path, content string, opts WriteOptions) (written, error) {
	if err := opts.Validate(); err != nil {
		return written{}, err
	}

	now := clock.Now()
	s := hash.XXH3(content)

	before, err := fts.Live(ctx, tx, path)
	if err != nil {
		return written{}, err
	}

	// A document is live or deleted as a whole. Writing to a deleted
	// path continues that document - same key, next version number -
	// so its earlier versions come back with it.
	res, err := tx.ExecContext(ctx, `
		UPDATE content SET deleted_at = NULL
		WHERE namespace = ? AND path = ? AND deleted_at IS NOT NULL
	`, namespace, path)
	if err != nil {
		return written{}, fmt.Errorf("restoring earlier versions: %w", err)
	}
	revived, err := res.RowsAffected()
	if err != nil {
		return written{}, fmt.Errorf("checking rows affected: %w", err)
	}

	// Check if content is unchanged from latest version
	var latest string
	err = tx.QueryRowContext(ctx, `
		SELECT hash FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).Scan(&latest)

	if err == nil && latest == s {
		// Content unchanged: no new version. A deleted document
		// written with its last content is simply restored.
		w := written{restored: revived > 0}
		if w.restored {
			if err := fts.Sync(ctx, tx, before, path); err != nil {
				return written{}, err
			}
		}
		w.doc, err = d.readInTx(ctx, tx, path)
		return w, err
	}

	// Get next version number
	var max int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM content
		WHERE namespace = ? AND path = ?
	`, namespace, path).Scan(&max)
	if err != nil {
		return written{}, fmt.Errorf("getting version: %w", err)
	}
	next := max + 1

	// Compute metadata
	m := meta.Compute(content)
	meta, err := json.Marshal(m)
	if err != nil {
		return written{}, fmt.Errorf("marshaling meta: %w", err)
	}

	// Reuse the document's existing key, or generate one for new documents.
	var k string
	err = tx.QueryRowContext(ctx, `
		SELECT key FROM content
		WHERE namespace = ? AND path = ?
		LIMIT 1
	`, namespace, path).Scan(&k)
	if err != nil {
		// A new document. Keys are shared by the versions of one path,
		// so the table cannot enforce uniqueness; check here instead,
		// inside the write transaction, that no other path holds it.
		k, err = key.Insert("content.key", func(k string) error {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM content WHERE key = ?`, k).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return errors.New("UNIQUE constraint failed: content.key")
			}
			return nil
		})
		if err != nil {
			return written{}, fmt.Errorf("choosing a key: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO content (
			key, namespace, path, content, version, hash,
			author, message, source, mime, meta, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, k, namespace, path, content, next, s,
		opts.Author, opts.Message, opts.Source, mime, string(meta), now)

	if err != nil {
		return written{}, fmt.Errorf("inserting document: %w", err)
	}

	if err := fts.Sync(ctx, tx, before, path); err != nil {
		return written{}, err
	}

	// Note: Events are emitted by caller after transaction commits

	return written{changed: true, doc: &document.Document{
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
	}}, nil
}
