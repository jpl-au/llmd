// add.go creates new tasks and their backing documents.

package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/jpl-au/llmd/errs"

	"github.com/jpl-au/llmd/clock"

	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/key"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/task"
)

// Add creates a new task and its backing document.
func (t *Tasks) Add(ctx context.Context, title string, body []byte, opts AddOptions) (*task.Task, error) {
	if title == "" {
		return nil, ErrMissingTitle
	}
	if opts.Priority < 0 {
		return nil, invalidPriority(opts.Priority)
	}
	if err := t.ensure(); err != nil {
		return nil, err
	}
	if err := t.ensureBoard(ctx, opts.Author); err != nil {
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
		return nil, errs.New(ErrInvalidCol, "there is no column %s. Run llmd task column list to see them.", status)
	}

	// Determine document path.
	//
	// --path links to an existing store document. The document must be live.
	// Body content (stdin/--file) creates a new document at tasks/<slug>.
	// No body and no --path: task has no spec, sits in backlog.
	var path string

	if opts.Path != "" {
		if err := t.docs.Check(ctx, opts.Path); err != nil {
			return nil, err
		}
		path = opts.Path
	} else {
		// Avoid squatting on an existing document or sharing a spec
		// path with another live task. If the auto-generated path is
		// taken, append a numeric suffix to make it unique.
		path, err = t.uniquePath(ctx, slug(title))
		if err != nil {
			return nil, err
		}

		if len(body) > 0 {
			_, err = t.docs.Write(ctx, path, string(body), documents.WriteOptions{
				Origin: opts.Origin,
			})
			if err != nil {
				return nil, fmt.Errorf("creating document: %w", err)
			}
		}
	}

	now := clock.Now()

	// Validate dependency target exists.
	if opts.DependsOn != "" {
		if _, err := t.Read(ctx, opts.DependsOn); err != nil {
			return nil, fmt.Errorf("dependency target: %w", err)
		}
	}

	var assignedTo sql.NullString
	if opts.AssignedTo != "" {
		assignedTo = sql.NullString{String: opts.AssignedTo, Valid: true}
	}
	var branch sql.NullString
	if opts.Branch != "" {
		branch = sql.NullString{String: opts.Branch, Valid: true}
	}

	result, err := t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		// Next position in the target column
		var maxPos int
		err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(position), -1) FROM tasks
			WHERE status = ? AND deleted_at IS NULL
		`, status).Scan(&maxPos)
		if err != nil {
			return nil, fmt.Errorf("getting position: %w", err)
		}

		k, err := key.Insert("tasks.key", func(k string) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO tasks (key, title, status, priority, position, assigned_to, branch, flags, depends_on, path, author, source, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?)
			`, k, title, status, opts.Priority, maxPos+1, assignedTo, branch, nullStr(opts.DependsOn), path, opts.Author, opts.Source, now)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("inserting task: %w", err)
		}

		recordTx(ctx, tx, opts.Author, "created", k, "", title)
		return added{k, maxPos + 1}, nil
	}).WithContext(ctx).Exec()
	if err != nil {
		return nil, err
	}

	k, pos := result.Value.(added).key, result.Value.(added).pos

	tsk := &task.Task{
		Key:        k,
		Title:      title,
		Status:     status,
		Priority:   opts.Priority,
		Position:   pos,
		AssignedTo: opts.AssignedTo,
		Branch:     opts.Branch,
		DependsOn:  opts.DependsOn,
		Path:       path,
		Origin:     opts.Origin,
		CreatedAt:  now,
	}

	if t.bus != nil {
		t.bus.Emit(ctx, events.Event{
			Type:      events.TaskCreated,
			Path:      path,
			Key:       k,
			Author:    opts.Author,
			Timestamp: now,
			Metadata:  map[string]any{"title": title, "status": status},
		})
	}

	return tsk, nil
}

// uniquePath returns tasks/<slug>, or the first tasks/<slug>-N (N from
// 2) that is used by neither a document nor a live task.
func (t *Tasks) uniquePath(ctx context.Context, s string) (string, error) {
	base := "tasks/" + s
	path := base
	for i := 2; ; i++ {
		taken, err := t.pathTaken(ctx, path)
		if err != nil {
			return "", err
		}
		if !taken {
			return path, nil
		}
		path = fmt.Sprintf("%s-%d", base, i)
	}
}

// pathTaken reports whether a document, live or deleted, holds path or
// a live task already claims it as its spec path. A deleted document
// still owns its path: writing a spec there would continue it.
func (t *Tasks) pathTaken(ctx context.Context, path string) (bool, error) {
	switch err := t.docs.Check(ctx, path); {
	case err == nil, errors.Is(err, documents.ErrDeleted):
		return true, nil
	case !errors.Is(err, documents.ErrNotFound):
		return false, err
	}

	var n int
	row, err := t.db.Query(`
		SELECT COUNT(*) FROM tasks WHERE path = ? AND deleted_at IS NULL
	`, path).WithContext(ctx).ReadRow()
	if err != nil {
		return false, fmt.Errorf("checking task path: %w", err)
	}
	if err := row.Scan(&n); err != nil {
		return false, fmt.Errorf("checking task path: %w", err)
	}
	return n > 0, nil
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

// added is what the insert transaction hands back: the key the store
// accepted and the position in the column.
type added struct {
	key string
	pos int
}
