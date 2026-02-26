// add.go creates new tasks and their backing documents.

package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/task"
)

// Add creates a new task and its backing document.
func (t *Tasks) Add(ctx context.Context, title string, body []byte, opts AddOptions) (*task.Task, error) {
	if title == "" {
		return nil, ErrMissingTitle
	}
	if err := t.ensure(); err != nil {
		return nil, err
	}
	if err := t.ensureBoard(ctx, opts.Author, opts.Source); err != nil {
		return nil, err
	}

	status := opts.Status
	if status == "" {
		status = "backlog"
	}

	// Validate column exists
	cols, err := t.Columns(ctx)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(cols, status) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCol, status)
	}

	// Determine document path.
	//
	// --path links to an existing store document. The document must exist.
	// Body content (stdin/--file) creates a new document at tasks/<slug>.
	// No body and no --path: task has no spec, sits in backlog.
	path := "tasks/" + slug(title)

	if opts.Path != "" {
		exists, eerr := t.docs.Exists(ctx, opts.Path)
		if eerr != nil {
			return nil, fmt.Errorf("checking document: %w", eerr)
		}
		if !exists {
			return nil, fmt.Errorf("document not found: %s", opts.Path)
		}
		path = opts.Path
	} else if len(body) > 0 {
		// Avoid squatting on an existing document. If the
		// auto-generated path is already taken, append a numeric
		// suffix to make it unique.
		for i := 2; ; i++ {
			exists, eerr := t.docs.Exists(ctx, path)
			if eerr != nil {
				return nil, fmt.Errorf("checking document: %w", eerr)
			}
			if !exists {
				break
			}
			path = fmt.Sprintf("tasks/%s-%d", slug(title), i)
		}

		_, err = t.docs.Write(ctx, path, string(body), documents.WriteOptions{
			Origin: opts.Origin,
		})
		if err != nil {
			return nil, fmt.Errorf("creating document: %w", err)
		}
	}

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Next position in the target column
	var maxPos int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), -1) FROM tasks
		WHERE status = ? AND deleted_at IS NULL
	`, status).Scan(&maxPos)
	if err != nil {
		return nil, fmt.Errorf("getting position: %w", err)
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	var assignedTo sql.NullString
	if opts.AssignedTo != "" {
		assignedTo = sql.NullString{String: opts.AssignedTo, Valid: true}
	}
	var branch sql.NullString
	if opts.Branch != "" {
		branch = sql.NullString{String: opts.Branch, Valid: true}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO tasks (key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?)
	`, k, title, status, opts.Priority, maxPos+1, assignedTo, branch, path, opts.Author, opts.Source, now)
	if err != nil {
		return nil, fmt.Errorf("inserting task: %w", err)
	}

	recordTx(ctx, tx, opts.Author, "created", k, "", title)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &task.Task{
		Key:        k,
		Title:      title,
		Status:     status,
		Priority:   opts.Priority,
		Position:   maxPos + 1,
		AssignedTo: opts.AssignedTo,
		Branch:     opts.Branch,
		Path:       path,
		Origin:     opts.Origin,
		CreatedAt:  now,
	}, nil
}

// slug converts a title to a URL-friendly path component.
func slug(title string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prev = r
		case prev != '-':
			b.WriteByte('-')
			prev = '-'
		}
	}
	s := b.String()
	return strings.TrimRight(s, "-")
}
