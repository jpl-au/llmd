// Package tasks provides task management.
//
// Tasks are stored in a dedicated table, created lazily on first use.
// Each task points to a document in the content table (the spec body).
// The history table logs every state change for observability.
//
// The tasks table is mutable - status, priority, position, assigned_to,
// and flags are updated in place. The audit package records what changed.
package tasks

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/lazy"
)

// Schema is the DDL for the tasks table and its indices.
const Schema = `
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    assigned_to TEXT,
    branch TEXT,
    flags TEXT,
    depends_on TEXT,
    path TEXT NOT NULL,
    author TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_key ON tasks(key);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_path ON tasks(path);
CREATE INDEX IF NOT EXISTS idx_tasks_deleted ON tasks(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_branch ON tasks(branch) WHERE branch IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_depends_on ON tasks(depends_on) WHERE depends_on IS NOT NULL;

CREATE TABLE IF NOT EXISTS board (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value TEXT NOT NULL,
    author TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
`

// DefaultColumns for a new board.
var DefaultColumns = []string{"backlog", "up-next", "in-progress", "review", "approval", "done", "blocked"}

var (
	ErrNotFound     = errors.New("the task does not exist.")
	ErrNoSpec       = errors.New("the task has no spec, so it cannot leave the backlog.")
	ErrInvalidCol   = errors.New("there is no such column.")
	ErrColNotEmpty  = errors.New("the column still has tasks. Move or delete them first.")
	ErrColExists    = errors.New("the column already exists.")
	ErrColNotFound  = errors.New("there is no such column.")
	ErrMissingTitle = errors.New("a title is required.")
	ErrCycle        = errors.New("that dependency would make a cycle.")
)

// Tasks provides task CRUD, board column management, and audit logging.
// The tasks table is created on first use when the work database is
// opened, so stores that never use tasks pay no schema cost.
type Tasks struct {
	dbFn  func() *qwr.Manager
	db    *qwr.Manager
	docs  *documents.Documents
	audit *audit.Log
	bus   *events.Bus
	order Order // ordering within a column; "" means OrderPriority
	once  lazy.Once
}

// New creates a Tasks instance with its dependencies. The db function
// returns the work database manager, opening it on demand if needed.
func New(db func() *qwr.Manager, docs *documents.Documents, audit *audit.Log, bus *events.Bus) *Tasks {
	return &Tasks{dbFn: db, docs: docs, audit: audit, bus: bus}
}

// ensure opens the work database and creates the tasks table if needed.
func (t *Tasks) ensure() error {
	return t.once.Do(func() error {
		db := t.dbFn()
		if db == nil {
			return fmt.Errorf("work database not available")
		}
		if _, err := db.Query(Schema).Exec(); err != nil {
			return err
		}
		// Ensure audit table exists for recordTx.
		if err := t.audit.Ensure(); err != nil {
			return err
		}
		t.db = db
		return nil
	})
}

// ensureBoard inserts the default board if no rows exist yet. A fresh
// store only gains the board when someone first creates a task.
func (t *Tasks) ensureBoard(ctx context.Context, author string) error {
	row, err := t.db.Query(`SELECT COUNT(*) FROM board`).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("checking board: %w", err)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("checking board: %w", err)
	}
	if count > 0 {
		return nil
	}
	return t.writeBoard(ctx, &Board{Columns: DefaultColumns}, author)
}
