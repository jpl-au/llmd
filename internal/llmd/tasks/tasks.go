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
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/pkg/model/core"
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
CREATE INDEX IF NOT EXISTS idx_tasks_branch ON tasks(branch) WHERE branch IS NOT NULL;
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
		// Ensure audit table exists for recordTx.
		t.err = t.audit.Ensure()
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
