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
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/qwr"
)

// Schema is the DDL for the tasks table and its indices.
const Schema = `
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
    depends_on TEXT,
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
CREATE INDEX IF NOT EXISTS idx_tasks_depends_on ON tasks(depends_on) WHERE depends_on IS NOT NULL;
`

// Default columns for a new board.
var DefaultColumns = []string{"backlog", "up-next", "in-progress", "review", "approval", "done", "blocked"}

const boardNamespace = "task:board"

var (
	ErrNotFound     = errors.New("task not found")
	ErrNoSpec       = errors.New("task has no spec")
	ErrInvalidCol   = errors.New("unknown column")
	ErrColNotEmpty  = errors.New("column has tasks - move or delete them first")
	ErrColExists    = errors.New("column already exists")
	ErrColNotFound  = errors.New("column not found")
	ErrMissingTitle = errors.New("title is required")
	ErrCycle        = errors.New("dependency cycle detected")
)

// Tasks provides task CRUD, board column management, and audit logging.
// The tasks table is created on first use when the work database is
// opened, so stores that never use tasks pay no schema cost.
type Tasks struct {
	dbFn     func() *qwr.Manager
	db       *qwr.Manager
	docs     *documents.Documents
	entities *entities.Entities
	audit    *audit.Log
	bus      *events.Bus
	once     sync.Once
	err      error
}

// New creates a Tasks instance with its dependencies. The db function
// returns the work database manager, opening it on demand if needed.
func New(db func() *qwr.Manager, docs *documents.Documents, ents *entities.Entities, audit *audit.Log, bus *events.Bus) *Tasks {
	return &Tasks{dbFn: db, docs: docs, entities: ents, audit: audit, bus: bus}
}

// ensure opens the work database and creates the tasks table if needed.
func (t *Tasks) ensure() error {
	t.once.Do(func() {
		t.db = t.dbFn()
		if t.db == nil {
			t.err = fmt.Errorf("work database not available")
			return
		}
		_, t.err = t.db.Query(Schema).Write()
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
	cols := `{"columns":["backlog","up-next","in-progress","review","approval","done","blocked"]}`
	_, err = t.entities.Write(ctx, boardNamespace, cols, entities.WriteOptions{
		Origin: core.Origin{Author: author, Source: source},
	})
	return err
}
