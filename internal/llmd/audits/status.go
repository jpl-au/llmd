// status.go provides the agent inbox query.

package audits

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

// Status returns pending audit threads assigned to the given author.
// A thread is pending when its effective status (latest entry) is
// "pending" or "needs-work" and the effective assignee matches the
// queried author.
func (a *Audits) Status(ctx context.Context, author string, sinceMS int64) (*StatusResult, error) {
	if author == "" {
		return nil, ErrMissingAuthor
	}
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// Find top-level audits where:
	// 1. The thread's effective status is pending or needs-work
	// 2. The effective assignee (latest entry) matches the author
	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT ` + columns + ` FROM audits AS top
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
			SELECT assignee FROM audits
			WHERE (id = top.id OR parent_id = top.id)
			  AND deleted_at IS NULL
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		  ) = ?
	`)
	args = append(args, author)

	if sinceMS > 0 {
		query.WriteString(" AND top.created_at > ?")
		args = append(args, sinceMS)
	}

	query.WriteString(" ORDER BY top.created_at DESC")

	rows, err := a.db.Query(query.String(), args...).WithContext(ctx).Read()
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
// thread. Falls back to "pending" if the query fails, logging the
// error so it is not silently lost.
func (a *Audits) effectiveStatus(ctx context.Context, threadID string) string {
	var status string
	row, err := a.db.Query(`
		SELECT status FROM audits
		WHERE (id = ? OR parent_id = ?)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, threadID, threadID).WithContext(ctx).ReadRow()
	if err != nil {
		slog.Debug("effectiveStatus: query failed", "thread", threadID, "err", err)
		return "pending"
	}
	if err := row.Scan(&status); err != nil {
		slog.Debug("effectiveStatus: scan failed", "thread", threadID, "err", err)
		return "pending"
	}
	return status
}
