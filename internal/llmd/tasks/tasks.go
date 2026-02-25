// Package tasks provides task management.
//
// Tasks are stored in a dedicated table, created lazily on first use.
// Each task points to a document in the content table (the spec body).
// The history table logs every state change for observability.
//
// The tasks table is mutable — status, priority, position, assigned_to,
// and flags are updated in place. The audit package records what changed.
package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/key"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/task"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    assigned_to TEXT,
    branch TEXT,
    flags TEXT,
    path TEXT NOT NULL,
    author TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_tasks_key ON tasks(key);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_path ON tasks(path);
CREATE INDEX IF NOT EXISTS idx_tasks_deleted ON tasks(deleted_at) WHERE deleted_at IS NOT NULL;
`

// Default columns for a new board.
var DefaultColumns = []string{"backlog", "up-next", "in-progress", "review", "done"}

const boardNamespace = "task:board"

var (
	ErrNotFound     = errors.New("task not found")
	ErrNoSpec       = errors.New("task has no spec")
	ErrInvalidCol   = errors.New("unknown column")
	ErrColNotEmpty  = errors.New("column has tasks — move or delete them first")
	ErrColExists    = errors.New("column already exists")
	ErrColNotFound  = errors.New("column not found")
	ErrMissingTitle = errors.New("title is required")
)

// Tasks provides task CRUD, board column management, and audit logging.
// The tasks table and board entity are created lazily on first use via
// sync.Once, so stores that never use tasks pay no schema cost.
type Tasks struct {
	db       *sql.DB
	docs     *documents.Documents
	entities *entities.Entities
	audit    *audit.Log
	once     sync.Once
	err      error
}

// New creates a Tasks instance with its dependencies. The tasks table
// is not created until the first operation that requires it (Add, Read,
// List, etc.), triggered by the ensure() call in each method.
func New(db *sql.DB, docs *documents.Documents, ents *entities.Entities, audit *audit.Log) *Tasks {
	return &Tasks{db: db, docs: docs, entities: ents, audit: audit}
}

// ensure creates the tasks table if it does not exist.
func (t *Tasks) ensure() error {
	t.once.Do(func() {
		_, t.err = t.db.Exec(schema)
		if t.err != nil {
			return
		}
		// Migration: add branch column for existing databases.
		_, _ = t.db.Exec("ALTER TABLE tasks ADD COLUMN branch TEXT")
	})
	return t.err
}

// ensureBoard creates the default board entity if one does not already
// exist. The board is stored as an entity in the "task:board" namespace
// with a JSON value like {"columns":["backlog","up-next",...]}. This
// lazy creation means a fresh store only gains the board entity when
// someone first creates a task.
func (t *Tasks) ensureBoard(ctx context.Context, author, source string) error {
	exists, err := t.entities.ExistsInNamespace(ctx, boardNamespace, "")
	if err != nil {
		return fmt.Errorf("checking board: %w", err)
	}
	if exists {
		return nil
	}
	cols := `{"columns":["backlog","up-next","in-progress","review","done"]}`
	_, err = t.entities.Write(ctx, boardNamespace, cols, entities.WriteOptions{
		Origin: core.Origin{Author: author, Source: source},
	})
	return err
}

// AddOptions configures a task add operation.
type AddOptions struct {
	core.Origin
	Status     string
	Priority   int
	AssignedTo string
	Branch     string
	Path       string // Existing store document to use as spec
}

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
		_, err = t.docs.Write(ctx, path, string(body), documents.WriteOptions{
			Origin: opts.Origin,
		})
		if err != nil {
			return nil, fmt.Errorf("creating document: %w", err)
		}
	}

	// Next position in the target column
	var maxPos int
	err = t.db.QueryRowContext(ctx, `
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

	_, err = t.db.ExecContext(ctx, `
		INSERT INTO tasks (key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?)
	`, k, title, status, opts.Priority, maxPos+1, assignedTo, branch, path, opts.Author, opts.Source, now)

	if err != nil {
		return nil, fmt.Errorf("inserting task: %w", err)
	}

	tsk := &task.Task{
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
	}

	_ = t.audit.Record(ctx, opts.Author, "created", k, "", title)

	return tsk, nil
}

// Read returns a task by key.
func (t *Tasks) Read(ctx context.Context, key string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	return t.scan(t.db.QueryRowContext(ctx, `
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE key = ? AND deleted_at IS NULL
	`, key))
}

// ListOptions filters the task listing. All zero-valued fields mean
// "no filter" — the query returns all non-deleted tasks. Filters are
// combined with AND.
type ListOptions struct {
	Status     string
	AssignedTo string
	Priority   int // 0 = all
}

// List returns all non-deleted tasks matching the filter criteria.
// Results are ordered by position then creation time. When no filters
// are set, returns every active task on the board.
func (t *Tasks) List(ctx context.Context, opts ListOptions) ([]*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE deleted_at IS NULL
	`)

	if opts.Status != "" {
		query.WriteString(" AND status = ?")
		args = append(args, opts.Status)
	}
	if opts.AssignedTo != "" {
		query.WriteString(" AND assigned_to = ?")
		args = append(args, opts.AssignedTo)
	}
	if opts.Priority > 0 {
		query.WriteString(" AND priority = ?")
		args = append(args, opts.Priority)
	}

	query.WriteString(" ORDER BY position ASC, created_at ASC")

	rows, err := t.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*task.Task
	for rows.Next() {
		tsk, err := t.scanRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, tsk)
	}
	return tasks, rows.Err()
}

// Move changes a task's status (column).
func (t *Tasks) Move(ctx context.Context, key, status, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return err
	}

	// Validate column
	cols, err := t.Columns(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(cols, status) {
		return fmt.Errorf("%w: %s", ErrInvalidCol, status)
	}

	// Spec gating: cannot leave backlog without content
	if tsk.Status == "backlog" && status != "backlog" {
		specced, err := t.hasSpec(ctx, tsk.Path)
		if err != nil {
			return err
		}
		if !specced {
			return ErrNoSpec
		}
	}

	oldStatus := tsk.Status

	// Next position in target column
	var maxPos int
	err = t.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), -1) FROM tasks
		WHERE status = ? AND deleted_at IS NULL
	`, status).Scan(&maxPos)
	if err != nil {
		return fmt.Errorf("getting position: %w", err)
	}

	_, err = t.db.ExecContext(ctx, `
		UPDATE tasks SET status = ?, position = ? WHERE key = ? AND deleted_at IS NULL
	`, status, maxPos+1, key)
	if err != nil {
		return fmt.Errorf("moving task: %w", err)
	}

	_ = t.audit.Record(ctx, author, "moved", key, oldStatus, status)
	return nil
}

// SetOptions configures which fields to update.
type SetOptions struct {
	Title      *string
	Priority   *int
	Position   *int
	AssignedTo *string
	Branch     *string
	Flag       string // Flag to add
	Unflag     string // Flag to remove
}

// Set updates task metadata.
func (t *Tasks) Set(ctx context.Context, key, author string, opts SetOptions) error {
	if err := t.ensure(); err != nil {
		return err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return err
	}

	if opts.Title != nil {
		old := tsk.Title
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET title = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Title, key)
		if err != nil {
			return fmt.Errorf("setting title: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:title", key, old, *opts.Title)
	}

	if opts.Priority != nil {
		old := fmt.Sprintf("%d", tsk.Priority)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET priority = ? WHERE key = ? AND deleted_at IS NULL`, *opts.Priority, key)
		if err != nil {
			return fmt.Errorf("setting priority: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:priority", key, old, fmt.Sprintf("%d", *opts.Priority))
	}

	if opts.AssignedTo != nil {
		old := tsk.AssignedTo
		var v sql.NullString
		if *opts.AssignedTo != "" {
			v = sql.NullString{String: *opts.AssignedTo, Valid: true}
		}
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET assigned_to = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
		if err != nil {
			return fmt.Errorf("setting assigned_to: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:assigned_to", key, old, *opts.AssignedTo)
	}

	if opts.Branch != nil {
		old := tsk.Branch
		var v sql.NullString
		if *opts.Branch != "" {
			v = sql.NullString{String: *opts.Branch, Valid: true}
		}
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET branch = ? WHERE key = ? AND deleted_at IS NULL`, v, key)
		if err != nil {
			return fmt.Errorf("setting branch: %w", err)
		}
		_ = t.audit.Record(ctx, author, "edited:branch", key, old, *opts.Branch)
	}

	if opts.Position != nil {
		old := fmt.Sprintf("%d", tsk.Position)
		if err := t.reposition(ctx, key, tsk.Status, *opts.Position, author); err != nil {
			return err
		}
		_ = t.audit.Record(ctx, author, "edited:position", key, old, fmt.Sprintf("%d", *opts.Position))
	}

	if opts.Flag != "" {
		old := tsk.Flags
		flags := addFlag(tsk.Flags, opts.Flag)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
		if err != nil {
			return fmt.Errorf("setting flag: %w", err)
		}
		_ = t.audit.Record(ctx, author, "flagged", key, old, flags)
	}

	if opts.Unflag != "" {
		old := tsk.Flags
		flags := removeFlag(tsk.Flags, opts.Unflag)
		_, err = t.db.ExecContext(ctx, `UPDATE tasks SET flags = ? WHERE key = ? AND deleted_at IS NULL`, nullStr(flags), key)
		if err != nil {
			return fmt.Errorf("removing flag: %w", err)
		}
		_ = t.audit.Record(ctx, author, "unflagged", key, old, flags)
	}

	return nil
}

// Delete soft-deletes a task.
func (t *Tasks) Delete(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	tsk, err := t.Read(ctx, key)
	if err != nil {
		return nil, err
	}

	now := time.Now().UnixMilli()
	_, err = t.db.ExecContext(ctx, `
		UPDATE tasks SET deleted_at = ? WHERE key = ? AND deleted_at IS NULL
	`, now, key)
	if err != nil {
		return nil, fmt.Errorf("deleting task: %w", err)
	}

	_ = t.audit.Record(ctx, author, "deleted", key, tsk.Status, "")
	return tsk, nil
}

// Restore undeletes a soft-deleted task.
func (t *Tasks) Restore(ctx context.Context, key, author string) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	// Read including deleted
	tsk, err := t.scanDeleted(ctx, key)
	if err != nil {
		return nil, err
	}

	_, err = t.db.ExecContext(ctx, `
		UPDATE tasks SET deleted_at = NULL WHERE key = ? AND deleted_at IS NOT NULL
	`, key)
	if err != nil {
		return nil, fmt.Errorf("restoring task: %w", err)
	}

	_ = t.audit.Record(ctx, author, "restored", key, "", tsk.Status)
	return tsk, nil
}

// Log returns audit events for a task, newest first.
// If key is empty, returns all task history.
func (t *Tasks) Log(ctx context.Context, key string, limit int) ([]audit.Event, error) {
	if key != "" {
		// Verify task exists (including deleted)
		if _, err := t.Read(ctx, key); err != nil {
			if _, err := t.scanDeleted(ctx, key); err != nil {
				return nil, err
			}
		}
	}
	return t.audit.Query(ctx, key, limit)
}

// Columns returns the board columns in order.
func (t *Tasks) Columns(ctx context.Context) ([]string, error) {
	return t.readColumns(ctx)
}

// AddColumn adds a new column. If after is empty, appends to the end.
func (t *Tasks) AddColumn(ctx context.Context, name, after, author string) error {
	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColExists, name)
	}

	if after == "" {
		cols = append(cols, name)
	} else {
		idx := slices.Index(cols, after)
		if idx < 0 {
			return fmt.Errorf("%w: %s", ErrColNotFound, after)
		}
		cols = append(cols[:idx+1], append([]string{name}, cols[idx+1:]...)...)
	}

	return t.writeColumns(ctx, cols, author)
}

// RemoveColumn removes a column if it has no tasks.
func (t *Tasks) RemoveColumn(ctx context.Context, name, author string) error {
	if err := t.ensure(); err != nil {
		return err
	}

	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}

	// Check for tasks in this column
	var count int
	err = t.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE status = ? AND deleted_at IS NULL
	`, name).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking column: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %s (%d tasks)", ErrColNotEmpty, name, count)
	}

	var filtered []string
	for _, c := range cols {
		if c != name {
			filtered = append(filtered, c)
		}
	}

	return t.writeColumns(ctx, filtered, author)
}

// MoveColumn reorders a column to be after another column.
func (t *Tasks) MoveColumn(ctx context.Context, name, after, author string) error {
	cols, err := t.readColumns(ctx)
	if err != nil {
		return err
	}

	if !slices.Contains(cols, name) {
		return fmt.Errorf("%w: %s", ErrColNotFound, name)
	}
	if !slices.Contains(cols, after) {
		return fmt.Errorf("%w: %s", ErrColNotFound, after)
	}

	// Remove name from current position
	var filtered []string
	for _, c := range cols {
		if c != name {
			filtered = append(filtered, c)
		}
	}

	// Insert after target
	idx := slices.Index(filtered, after)
	result := append(filtered[:idx+1], append([]string{name}, filtered[idx+1:]...)...)

	return t.writeColumns(ctx, result, author)
}

// reposition moves a task to a specific position within its column,
// renumbering other tasks to maintain order.
func (t *Tasks) reposition(ctx context.Context, key, status string, pos int, _ string) error {
	// Get all tasks in this column, ordered by position
	rows, err := t.db.QueryContext(ctx, `
		SELECT key FROM tasks
		WHERE status = ? AND deleted_at IS NULL
		ORDER BY position ASC, created_at ASC
	`, status)
	if err != nil {
		return fmt.Errorf("listing column: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Remove the target key from the list
	var others []string
	for _, k := range keys {
		if k != key {
			others = append(others, k)
		}
	}

	// Clamp position
	if pos < 0 {
		pos = 0
	}
	if pos > len(others) {
		pos = len(others)
	}

	// Insert at desired position
	reordered := make([]string, 0, len(others)+1)
	reordered = append(reordered, others[:pos]...)
	reordered = append(reordered, key)
	reordered = append(reordered, others[pos:]...)

	// Update all positions
	for i, k := range reordered {
		_, err := t.db.ExecContext(ctx, `
			UPDATE tasks SET position = ? WHERE key = ? AND deleted_at IS NULL
		`, i, k)
		if err != nil {
			return fmt.Errorf("renumbering: %w", err)
		}
	}

	return nil
}

// hasSpec checks whether a task's document has real content beyond
// the template heading.
func (t *Tasks) hasSpec(ctx context.Context, path string) (bool, error) {
	doc, err := t.docs.Read(ctx, path)
	if err != nil {
		return false, nil // No document = no spec
	}
	// Strip the template heading and check if anything remains
	content := strings.TrimSpace(doc.Content)
	if idx := strings.Index(content, "\n"); idx >= 0 {
		after := strings.TrimSpace(content[idx:])
		return after != "", nil
	}
	// Single line = just the heading
	return false, nil
}

// readColumns reads the board columns from the entity.
func (t *Tasks) readColumns(ctx context.Context) ([]string, error) {
	exists, err := t.entities.ExistsInNamespace(ctx, boardNamespace, "")
	if err != nil {
		return nil, err
	}
	if !exists {
		return DefaultColumns, nil
	}

	ents, err := t.entities.List(ctx, boardNamespace, entities.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(ents) == 0 {
		return DefaultColumns, nil
	}

	// Parse the JSON value
	return parseColumns(ents[0].Value)
}

// writeColumns updates the board entity with new columns.
// Soft-deletes the old entity and creates a new one (insert-only).
func (t *Tasks) writeColumns(ctx context.Context, cols []string, author string) error {
	// Soft-delete existing board entity
	ents, err := t.entities.List(ctx, boardNamespace, entities.ListOptions{})
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := t.entities.Delete(ctx, e.Key, entities.DeleteOptions{
			Origin: core.Origin{Author: author, Source: "cli"},
		}); err != nil {
			return err
		}
	}

	// Write new one
	value := formatColumns(cols)
	_, err = t.entities.Write(ctx, boardNamespace, value, entities.WriteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
	return err
}

// scanDeleted reads a task including soft-deleted ones.
func (t *Tasks) scanDeleted(ctx context.Context, key string) (*task.Task, error) {
	return t.scan(t.db.QueryRowContext(ctx, `
		SELECT id, key, title, status, priority, position, assigned_to, branch, flags, path, author, source, created_at, deleted_at
		FROM tasks
		WHERE key = ?
		ORDER BY deleted_at DESC
		LIMIT 1
	`, key))
}

// scan reads a single task row from a sql.Row. It handles the nullable
// columns (assigned_to, branch, flags, deleted_at) by scanning into
// sql.Null types and converting to Go zero values. Returns ErrNotFound
// when no row matches.
func (t *Tasks) scan(row *sql.Row) (*task.Task, error) {
	var tsk task.Task
	var assignedTo, branch, flags sql.NullString
	var deletedAt sql.NullInt64

	err := row.Scan(
		&tsk.ID, &tsk.Key, &tsk.Title, &tsk.Status,
		&tsk.Priority, &tsk.Position, &assignedTo, &branch, &flags,
		&tsk.Path, &tsk.Author, &tsk.Source, &tsk.CreatedAt, &deletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		tsk.AssignedTo = assignedTo.String
	}
	if branch.Valid {
		tsk.Branch = branch.String
	}
	if flags.Valid {
		tsk.Flags = flags.String
	}
	if deletedAt.Valid {
		tsk.DeletedAt = &deletedAt.Int64
	}
	return &tsk, nil
}

// scanRow reads a single task from sql.Rows (used in List iterations).
// Same nullable handling as scan, but operates on Rows instead of Row.
func (t *Tasks) scanRow(rows *sql.Rows) (*task.Task, error) {
	var tsk task.Task
	var assignedTo, branch, flags sql.NullString
	var deletedAt sql.NullInt64

	err := rows.Scan(
		&tsk.ID, &tsk.Key, &tsk.Title, &tsk.Status,
		&tsk.Priority, &tsk.Position, &assignedTo, &branch, &flags,
		&tsk.Path, &tsk.Author, &tsk.Source, &tsk.CreatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		tsk.AssignedTo = assignedTo.String
	}
	if branch.Valid {
		tsk.Branch = branch.String
	}
	if flags.Valid {
		tsk.Flags = flags.String
	}
	if deletedAt.Valid {
		tsk.DeletedAt = &deletedAt.Int64
	}
	return &tsk, nil
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

// addFlag appends a flag to the comma-separated flags string. Returns
// the original string unchanged if the flag is already present.
func addFlag(flags, flag string) string {
	if flags == "" {
		return flag
	}
	for f := range strings.SplitSeq(flags, ",") {
		if f == flag {
			return flags // Already set
		}
	}
	return flags + "," + flag
}

// removeFlag removes a flag from the comma-separated flags string.
// Returns the remaining flags joined by commas, or empty string if
// no flags remain.
func removeFlag(flags, flag string) string {
	if flags == "" {
		return ""
	}
	var result []string
	for f := range strings.SplitSeq(flags, ",") {
		if f != flag {
			result = append(result, f)
		}
	}
	return strings.Join(result, ",")
}

// nullStr converts a Go string to sql.NullString. Empty strings become
// NULL in the database; non-empty strings become valid values.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// parseColumns extracts the columns array from the board entity JSON.
func parseColumns(value string) ([]string, error) {
	// Simple parsing — the value is {"columns":["a","b","c"]}
	// Use strings rather than encoding/json to avoid allocations for
	// this hot path in list operations.
	start := strings.Index(value, "[")
	end := strings.LastIndex(value, "]")
	if start < 0 || end < 0 || end <= start {
		return DefaultColumns, nil
	}

	inner := value[start+1 : end]
	if inner == "" {
		return nil, nil
	}

	var cols []string
	for part := range strings.SplitSeq(inner, ",") {
		col := strings.Trim(strings.TrimSpace(part), `"`)
		if col != "" {
			cols = append(cols, col)
		}
	}
	return cols, nil
}

// formatColumns serialises the column list to the JSON format stored in
// the board entity: {"columns":["backlog","up-next",...]}.
func formatColumns(cols []string) string {
	var quoted []string
	for _, c := range cols {
		quoted = append(quoted, `"`+c+`"`)
	}
	return `{"columns":[` + strings.Join(quoted, ",") + `]}`
}
