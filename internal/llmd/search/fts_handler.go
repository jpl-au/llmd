package search

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/qwr"
)

// FTSHandler maintains the FTS5 index in response to document events.
// The latest non-deleted version of each document is indexed.
type FTSHandler struct {
	db *qwr.Manager
}

// NewFTSHandler creates a new FTS event handler.
func NewFTSHandler(db *qwr.Manager) *FTSHandler {
	return &FTSHandler{db: db}
}

// HandleEvent processes document events to maintain the FTS index.
func (h *FTSHandler) HandleEvent(ctx context.Context, event events.Event) error {
	switch event.Type {
	case events.DocumentWritten:
		return h.onWrite(ctx, event)
	case events.DocumentDeleted:
		return h.onDelete(ctx, event)
	case events.DocumentRestored:
		return h.onRestore(ctx, event)
	case events.DocumentMoved:
		return h.onMove(ctx, event)
	}
	return nil
}

// onWrite adds/updates a document in the FTS index. The delete and
// insert run in a single transaction so a crash cannot leave the
// index missing the document.
func (h *FTSHandler) onWrite(ctx context.Context, event events.Event) error {
	// Get the row ID for the new version
	var rowID int64
	var key, path, content string
	row, err := h.db.Query(`
		SELECT id, key, path, content FROM content
		WHERE namespace = 'core:document' AND path = ? AND version = ?
	`, event.Path, event.Version).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("getting document for FTS: %w", err)
	}
	if err := row.Scan(&rowID, &key, &path, &content); err != nil {
		return fmt.Errorf("getting document for FTS: %w", err)
	}

	// For updates, gather the previous version data for the FTS delete
	var prevRowID int64
	var prevKey, prevPath, prevContent string
	var hasPrev bool
	if event.Version > 1 {
		prevRow, err := h.db.Query(`
			SELECT id, key, path, content FROM content
			WHERE namespace = 'core:document' AND path = ? AND version = ?
		`, event.Path, event.Version-1).WithContext(ctx).ReadRow()
		if err != nil {
			return fmt.Errorf("getting previous version for FTS: %w", err)
		}
		if err := prevRow.Scan(&prevRowID, &prevKey, &prevPath, &prevContent); err == nil {
			hasPrev = true
		}
	}

	_, err = h.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		if hasPrev {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO content_fts(content_fts, rowid, key, path, content)
				VALUES ('delete', ?, ?, ?, ?)
			`, prevRowID, prevKey, prevPath, prevContent)
			if err != nil {
				return nil, fmt.Errorf("removing previous version from FTS: %w", err)
			}
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO content_fts(rowid, key, path, content)
			VALUES (?, ?, ?, ?)
		`, rowID, key, path, content)
		if err != nil {
			return nil, fmt.Errorf("adding to FTS: %w", err)
		}

		return nil, nil
	}).WithContext(ctx).Write()

	return err
}

// onDelete removes a document from the FTS index.
func (h *FTSHandler) onDelete(ctx context.Context, event events.Event) error {
	// Get the latest version that was just deleted
	var rowID int64
	var key, path, content string
	row, err := h.db.Query(`
		SELECT id, key, path, content FROM content
		WHERE namespace = 'core:document' AND path = ? AND deleted_at IS NOT NULL
		ORDER BY version DESC LIMIT 1
	`, event.Path).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("getting deleted document for FTS: %w", err)
	}
	if err := row.Scan(&rowID, &key, &path, &content); err != nil {
		if err == sql.ErrNoRows {
			return nil // Already gone
		}
		return fmt.Errorf("getting deleted document for FTS: %w", err)
	}

	// Remove from FTS
	_, err = h.db.Query(`
		INSERT INTO content_fts(content_fts, rowid, key, path, content)
		VALUES ('delete', ?, ?, ?, ?)
	`, rowID, key, path, content).WithContext(ctx).Execute()
	if err != nil {
		return fmt.Errorf("removing from FTS: %w", err)
	}

	return nil
}

// onRestore adds a document back to the FTS index.
func (h *FTSHandler) onRestore(ctx context.Context, event events.Event) error {
	// Get the latest version
	var rowID int64
	var key, path, content string
	row, err := h.db.Query(`
		SELECT id, key, path, content FROM content
		WHERE namespace = 'core:document' AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, event.Path).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("getting restored document for FTS: %w", err)
	}
	if err := row.Scan(&rowID, &key, &path, &content); err != nil {
		return fmt.Errorf("getting restored document for FTS: %w", err)
	}

	// Add to FTS
	_, err = h.db.Query(`
		INSERT INTO content_fts(rowid, key, path, content)
		VALUES (?, ?, ?, ?)
	`, rowID, key, path, content).WithContext(ctx).Execute()
	if err != nil {
		return fmt.Errorf("adding restored document to FTS: %w", err)
	}

	return nil
}

// onMove updates the path in the FTS index. The delete and insert
// run in a single transaction so a crash cannot leave the index
// missing the document.
func (h *FTSHandler) onMove(ctx context.Context, event events.Event) error {
	oldPath, ok := event.Metadata["old_path"].(string)
	if !ok {
		return fmt.Errorf("move event missing old_path in metadata")
	}

	// Get the latest version at the new path
	var rowID int64
	var key, content string
	row, err := h.db.Query(`
		SELECT id, key, content FROM content
		WHERE namespace = 'core:document' AND path = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, event.Path).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("getting moved document for FTS: %w", err)
	}
	if err := row.Scan(&rowID, &key, &content); err != nil {
		return fmt.Errorf("getting moved document for FTS: %w", err)
	}

	_, err = h.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content_fts(content_fts, rowid, key, path, content)
			VALUES ('delete', ?, ?, ?, ?)
		`, rowID, key, oldPath, content)
		if err != nil {
			return nil, fmt.Errorf("removing old path from FTS: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO content_fts(rowid, key, path, content)
			VALUES (?, ?, ?, ?)
		`, rowID, key, event.Path, content)
		if err != nil {
			return nil, fmt.Errorf("adding new path to FTS: %w", err)
		}

		return nil, nil
	}).WithContext(ctx).Write()

	return err
}
