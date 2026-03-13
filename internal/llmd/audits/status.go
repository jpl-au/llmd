// status.go provides the agent inbox query.

package audits

import (
	"context"
	"fmt"
)

// StatusResult is the agent's inbox.
type StatusResult struct {
	Author  string
	Pending []Audit
	Summary Summary
}

// Summary provides aggregate counts.
type Summary struct {
	Total     int
	NeedsWork int
	Pending   int
}

// Status returns pending audit threads requiring the given author's
// attention. A thread requires attention when:
//   - The effective status is "pending" or "needs-work"
//   - The last entry in the thread is NOT from the given author
//
// Target ownership (document author, task assignee) is handled at the
// bridge layer where both AuditStore and TaskStore are available.
func (a *Audits) Status(ctx context.Context, author string) (*StatusResult, error) {
	if author == "" {
		return nil, ErrMissingAuthor
	}
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// Find top-level audits where:
	// 1. The thread's effective status is pending or needs-work
	// 2. The last entry in the thread is NOT from this author
	rows, err := a.db.QueryContext(ctx, `
		SELECT `+columns+` FROM audits AS top
		WHERE top.deleted_at IS NULL
		  AND top.parent_id IS NULL
		  AND (
			SELECT status FROM audits
			WHERE (id = top.id OR parent_id = top.id)
			  AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		  ) IN ('pending', 'needs-work')
		  AND (
			SELECT author FROM audits
			WHERE (id = top.id OR parent_id = top.id)
			  AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		  ) != ?
		ORDER BY top.created_at DESC
	`, author)
	if err != nil {
		return nil, fmt.Errorf("querying audit status: %w", err)
	}
	defer rows.Close()

	pending, err := scanAll(rows)
	if err != nil {
		return nil, err
	}

	var summary Summary
	summary.Total = len(pending)
	for _, aud := range pending {
		// Use the effective status (latest entry) for counting.
		// Since we filtered for pending/needs-work, we need to
		// re-derive from the thread. For simplicity, query the
		// effective status per thread.
		effStatus := a.effectiveStatus(ctx, aud.ID)
		switch effStatus {
		case "needs-work":
			summary.NeedsWork++
		case "pending":
			summary.Pending++
		}
	}

	return &StatusResult{
		Author:  author,
		Pending: pending,
		Summary: summary,
	}, nil
}

// effectiveStatus returns the status of the most recent entry in a
// thread. Falls back to "pending" on error.
func (a *Audits) effectiveStatus(ctx context.Context, threadID string) string {
	var status string
	err := a.db.QueryRowContext(ctx, `
		SELECT status FROM audits
		WHERE (id = ? OR parent_id = ?)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, threadID, threadID).Scan(&status)
	if err != nil {
		return "pending"
	}
	return status
}
