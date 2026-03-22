package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// Stat represents document metadata without content. It mirrors the
// Document struct but omits the Content field, making it cheaper to
// retrieve when only metadata is needed (e.g. checking existence,
// comparing hashes, or building listings).
type Stat struct {
	ID        int64
	Key       string
	Path      string
	Version   int
	Hash      string
	Author    string
	Message   string
	Source    string
	MIME      string
	Meta      *document.Meta
	CreatedAt int64
	DeletedAt *int64
	Resolved  document.Resolved
}

// Stat returns document metadata without loading content.
// This is more efficient than Read when only metadata is needed.
// value can be a document path or 9-char key.
// Checks path and key (if 9 chars) concurrently, returns with priority: path > key.
func (d *Documents) Stat(ctx context.Context, value string) (*Stat, error) {
	var wg sync.WaitGroup
	var pathStat, keyStat *Stat
	var pathErr, keyErr error

	// Check by path
	wg.Go(func() {
		pathStat, pathErr = d.statByPath(ctx, value)
	})

	// Check by key (only if 9 chars)
	wg.Go(func() {
		if len(value) != 9 {
			return
		}
		keyStat, keyErr = d.statByKey(ctx, value)
	})

	wg.Wait()

	// Return by priority: path > key
	if pathStat != nil {
		return pathStat, pathErr
	}
	if keyStat != nil {
		return keyStat, keyErr
	}

	// Return the most relevant error
	if pathErr != nil && pathErr != ErrNotFound {
		return nil, pathErr
	}
	if keyErr != nil && keyErr != ErrNotFound {
		return nil, keyErr
	}
	return nil, ErrNotFound
}

// statByKey queries the content table by the document's stable key.
// Returns the latest version for that key regardless of soft-delete status.
func (d *Documents) statByKey(ctx context.Context, key string) (*Stat, error) {
	var stat Stat
	var meta sql.NullString
	var message sql.NullString
	var mime sql.NullString
	var deletedAt sql.NullInt64

	row, err := d.db.Query(`
		SELECT id, key, path, version, hash, author, message, source, mime, meta, created_at, deleted_at
		FROM content
		WHERE namespace = ? AND key = ?
	`, namespace, key).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("querying stat by key: %w", err)
	}

	err = row.Scan(
		&stat.ID, &stat.Key, &stat.Path, &stat.Version, &stat.Hash,
		&stat.Author, &message, &stat.Source, &mime, &meta,
		&stat.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning stat by key: %w", err)
	}

	stat.Resolved = document.ResolvedKey
	return finish(&stat, message, mime, meta, deletedAt)
}

// statByPath queries the content table by document path. Returns only
// the latest non-deleted version (unlike statByKey which includes deleted).
func (d *Documents) statByPath(ctx context.Context, path string) (*Stat, error) {
	var stat Stat
	var meta sql.NullString
	var message sql.NullString
	var mime sql.NullString
	var deletedAt sql.NullInt64

	row, err := d.db.Query(`
		SELECT id, key, path, version, hash, author, message, source, mime, meta, created_at, deleted_at
		FROM content
		WHERE namespace = ? AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, fmt.Errorf("querying stat by path: %w", err)
	}

	err = row.Scan(
		&stat.ID, &stat.Key, &stat.Path, &stat.Version, &stat.Hash,
		&stat.Author, &message, &stat.Source, &mime, &meta,
		&stat.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning stat by path: %w", err)
	}

	stat.Resolved = document.ResolvedPath
	return finish(&stat, message, mime, meta, deletedAt)
}

// finish populates nullable Stat fields from their sql.Null wrappers
// and returns the appropriate error. If the document is soft-deleted
// (deletedAt is valid), finish returns the Stat alongside ErrDeleted  -
// callers can still inspect the metadata.
func finish(stat *Stat, message, mime, meta sql.NullString, deletedAt sql.NullInt64) (*Stat, error) {
	if message.Valid {
		stat.Message = message.String
	}
	if mime.Valid {
		stat.MIME = mime.String
	}
	if deletedAt.Valid {
		stat.DeletedAt = &deletedAt.Int64
		return stat, ErrDeleted
	}

	if meta.Valid && meta.String != "" {
		var m document.Meta
		if err := json.Unmarshal([]byte(meta.String), &m); err == nil {
			stat.Meta = &m
		}
	}

	return stat, nil
}
