// list.go handles listing audits with filters.

package audits

import (
	"context"
	"fmt"
	"strings"
)

// ListOptions filters the audit listing.
type ListOptions struct {
	Target   string
	ByAuthor string // filter by who created the audit
	Assignee string // filter by who the audit is assigned to
	Status   string
	Pending  bool  // shorthand: status in (pending, needs-work)
	SinceMS  int64 // Unix millis; 0 = no filter
}

// List returns non-deleted audits matching the filter criteria. When
// Pending or Status filters are set, they apply to the thread's
// effective status (the status of the most recent entry in each thread).
//
// Results are ordered newest first.
func (a *Audits) List(ctx context.Context, opts ListOptions) ([]Audit, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}

	// When filtering by status, we need thread-level effective status.
	// For unfiltered or target/author-only queries, return top-level
	// audits directly.
	if opts.Status != "" || opts.Pending {
		return a.listByEffectiveStatus(ctx, opts)
	}

	var query strings.Builder
	var args []any

	query.WriteString(`SELECT ` + columns + ` FROM audits WHERE deleted_at IS NULL AND parent_id IS NULL`)

	if opts.Target != "" {
		query.WriteString(" AND target = ?")
		args = append(args, opts.Target)
	}
	if opts.ByAuthor != "" {
		query.WriteString(" AND author = ?")
		args = append(args, opts.ByAuthor)
	}
	if opts.Assignee != "" {
		query.WriteString(" AND assignee = ?")
		args = append(args, opts.Assignee)
	}
	if opts.SinceMS > 0 {
		query.WriteString(" AND created_at > ?")
		args = append(args, opts.SinceMS)
	}

	query.WriteString(" ORDER BY created_at DESC")

	rows, err := a.db.Query(query.String(), args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing audits: %w", err)
	}
	defer rows.Close()

	return scanAll(rows)
}

// listByEffectiveStatus returns top-level audits whose thread's latest
// entry matches the status filter.
func (a *Audits) listByEffectiveStatus(ctx context.Context, opts ListOptions) ([]Audit, error) {
	// Subquery finds the effective status for each thread: the status
	// of the most recent non-deleted entry.
	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT ` + columns + ` FROM audits AS top
		WHERE top.deleted_at IS NULL
		  AND top.parent_id IS NULL
	`)

	if opts.Target != "" {
		query.WriteString(" AND top.target = ?")
		args = append(args, opts.Target)
	}
	if opts.ByAuthor != "" {
		query.WriteString(" AND top.author = ?")
		args = append(args, opts.ByAuthor)
	}
	if opts.Assignee != "" {
		query.WriteString(" AND top.assignee = ?")
		args = append(args, opts.Assignee)
	}
	if opts.SinceMS > 0 {
		query.WriteString(" AND top.created_at > ?")
		args = append(args, opts.SinceMS)
	}

	// Effective status subquery: latest entry in each thread.
	query.WriteString(` AND (
		SELECT status FROM audits
		WHERE (id = top.id OR parent_id = top.id)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	)`)

	if opts.Pending {
		query.WriteString(" IN ('pending', 'needs-work')")
	} else {
		query.WriteString(" = ?")
		args = append(args, opts.Status)
	}

	query.WriteString(" ORDER BY top.created_at DESC")

	rows, err := a.db.Query(query.String(), args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing audits by status: %w", err)
	}
	defer rows.Close()

	return scanAll(rows)
}

// scanAll reads all rows into a slice of Audit.
func scanAll(rows interface {
	Next() bool
	Err() error
	Scan(...any) error
},
) ([]Audit, error) {
	var result []Audit
	for rows.Next() {
		entry, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *entry)
	}
	return result, rows.Err()
}
