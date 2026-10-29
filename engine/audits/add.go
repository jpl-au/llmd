// add.go creates new audit entries.

package audits

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/clock"

	"github.com/jpl-au/llmd/engine/key"
	"github.com/jpl-au/llmd/pkg/events"
)

// AddOptions configures an audit add operation.
type AddOptions struct {
	Target   string
	Content  string
	Author   string
	Assignee string
	Status   string
	Version  int
}

// Add creates a top-level audit on a document or task. The target_type
// is inferred from the target: valid 10-char base36 keys are tasks,
// everything else is a document path.
func (a *Audits) Add(ctx context.Context, opts AddOptions) (*Audit, error) {
	if opts.Author == "" {
		return nil, ErrMissingAuthor
	}
	if opts.Target == "" {
		return nil, ErrMissingTarget
	}
	if err := a.ensure(); err != nil {
		return nil, err
	}

	status := opts.Status
	if status == "" {
		status = "pending"
	}

	targetType := inferTargetType(opts.Target)

	now := clock.Now()

	var version *int
	if opts.Version > 0 {
		version = &opts.Version
	}

	id, err := key.Insert("audits.id", func(id string) error {
		_, err := a.db.Query(`
			INSERT INTO audits (id, target, target_type, version, author, assignee, status, content, parent_id, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)
		`, id, opts.Target, targetType, version, opts.Author, opts.Assignee, status, opts.Content, now).WithContext(ctx).Exec()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("inserting audit: %w", err)
	}

	result := &Audit{
		ID:         id,
		Target:     opts.Target,
		TargetType: targetType,
		Version:    opts.Version,
		Author:     opts.Author,
		Assignee:   opts.Assignee,
		Status:     status,
		Content:    opts.Content,
		CreatedAt:  now,
	}

	if a.bus != nil {
		a.bus.Emit(ctx, events.Event{
			Type:      events.AuditCreated,
			Path:      opts.Target,
			Key:       id,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"assignee": opts.Assignee, "status": status},
		})
	}

	return result, nil
}

// inferTargetType returns "task" if the target looks like a valid task
// key (10-char base36), otherwise "document".
func inferTargetType(target string) string {
	if key.Validate(target) == nil {
		return "task"
	}
	return "document"
}
