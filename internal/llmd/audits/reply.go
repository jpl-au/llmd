// reply.go adds responses to existing audit threads.

package audits

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/events"
)

// Reply adds a response to an existing audit thread. If the parent is
// itself a reply, the store resolves to the top-level ancestor so all
// threads remain single-level.
func (a *Audits) Reply(ctx context.Context, parentID string, opts AddOptions) (*Audit, error) {
	if opts.Author == "" {
		return nil, ErrMissingAuthor
	}
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// Resolve to top-level parent.
	parent, err := a.read(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("reading parent: %w", err)
	}

	threadID := parent.ID
	if parent.ParentID != "" {
		threadID = parent.ParentID
	}

	status := opts.Status
	if status == "" {
		status = parent.Status
	}

	assignee := opts.Assignee
	if assignee == "" {
		assignee = parent.Assignee
	}

	id := key.Generate()
	now := time.Now().UnixMilli()

	_, err = a.db.Query(`
		INSERT INTO audits (id, target, target_type, version, author, assignee, status, content, parent_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, parent.Target, parent.TargetType, parent.Version, opts.Author, assignee, status, opts.Content, threadID, now).WithContext(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("inserting reply: %w", err)
	}

	result := &Audit{
		ID:         id,
		Target:     parent.Target,
		TargetType: parent.TargetType,
		Version:    parent.Version,
		Author:     opts.Author,
		Assignee:   assignee,
		Status:     status,
		Content:    opts.Content,
		ParentID:   threadID,
		CreatedAt:  now,
	}

	if a.bus != nil {
		eventType := events.AuditReplied
		if status == "approved" {
			eventType = events.AuditResolved
		}
		if err := a.bus.Emit(ctx, events.Event{
			Type:      eventType,
			Path:      parent.Target,
			Key:       id,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"parent_id": threadID, "status": status},
		}); err != nil {
			return nil, fmt.Errorf("emitting event: %w", err)
		}
	}

	return result, nil
}

// read fetches a single audit by ID, including soft-deleted.
func (a *Audits) read(ctx context.Context, id string) (*Audit, error) {
	row, err := a.db.Query(`
		SELECT `+columns+` FROM audits WHERE id = ?
	`, id).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	aud, err := scanAudit(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return aud, err
}
