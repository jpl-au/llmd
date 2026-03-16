// resolve.go handles resolving audits and soft-deleting.

package audits

import (
	"context"
	"fmt"
	"time"
)

// Resolve inserts a new entry with status "approved" and empty content.
// Internally equivalent to Reply with status approved.
func (a *Audits) Resolve(ctx context.Context, id, author string) (*Audit, error) {
	return a.Reply(ctx, id, AddOptions{
		Author: author,
		Status: "approved",
	})
}

// Restore undeletes a soft-deleted audit by clearing deleted_at.
func (a *Audits) Restore(ctx context.Context, id, author string) (*Audit, error) {
	if author == "" {
		return nil, ErrMissingAuthor
	}
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// Read including deleted.
	row, err := a.db.Query(`
		SELECT `+columns+` FROM audits WHERE id = ?
	`, id).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	aud, err := scanAudit(row)
	if err != nil {
		return nil, ErrNotFound
	}

	_, err = a.db.Query(`
		UPDATE audits SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL
	`, id).WithContext(ctx).Execute()
	if err != nil {
		return nil, fmt.Errorf("restoring audit: %w", err)
	}
	return aud, nil
}

// Delete soft-deletes an audit by setting deleted_at.
func (a *Audits) Delete(ctx context.Context, id, author string) error {
	if author == "" {
		return ErrMissingAuthor
	}
	if err := a.ensure(); err != nil {
		return err
	}

	// Verify the audit exists.
	_, err := a.Read(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	_, err = a.db.Query(`
		UPDATE audits SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL
	`, now, id).WithContext(ctx).Execute()
	if err != nil {
		return fmt.Errorf("deleting audit: %w", err)
	}
	return nil
}
