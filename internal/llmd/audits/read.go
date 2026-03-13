// read.go handles reading individual audits and threads.

package audits

import (
	"context"
	"database/sql"
	"fmt"
)

// Read returns a single non-deleted audit by ID.
func (a *Audits) Read(ctx context.Context, id string) (*Audit, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	row := a.db.QueryRowContext(ctx, `
		SELECT `+columns+` FROM audits
		WHERE id = ? AND deleted_at IS NULL
	`, id)
	aud, err := scanAudit(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return aud, err
}

// Thread returns a top-level audit and all its replies in chronological
// order. If the given ID is a reply, it resolves to the top-level
// parent first.
func (a *Audits) Thread(ctx context.Context, id string) ([]Audit, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// Resolve to top-level parent if needed.
	aud, err := a.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	threadID := aud.ID
	if aud.ParentID != "" {
		threadID = aud.ParentID
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT `+columns+` FROM audits
		WHERE (id = ? OR parent_id = ?) AND deleted_at IS NULL
		ORDER BY created_at ASC
	`, threadID, threadID)
	if err != nil {
		return nil, fmt.Errorf("querying thread: %w", err)
	}
	defer rows.Close()

	var thread []Audit
	for rows.Next() {
		entry, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		thread = append(thread, *entry)
	}
	return thread, rows.Err()
}
