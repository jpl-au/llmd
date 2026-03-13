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
	_, err = a.db.ExecContext(ctx, `
		UPDATE audits SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL
	`, now, id)
	if err != nil {
		return fmt.Errorf("deleting audit: %w", err)
	}
	return nil
}
