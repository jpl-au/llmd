// Package agents manages agent configurations and execution records.
//
// Agent configurations, prompt templates, and runtime settings are
// stored as plain files under .llmd/agents/:
//
//	.llmd/agents/
//	  claude-code/
//	    config.json        operational config
//	    settings.json      runtime settings (permissions, hooks)
//	    developer.md       prompt template
//	    auditor.md         prompt template
//	  default/
//	    developer.md       fallback prompt templates
//	    auditor.md
//
// Files are seeded from embedded assets on registration and can be
// edited directly. The filesystem is the source of truth.
//
// Agent activity (run tracking) is stored in SQLite for querying.
//
// This package handles data operations only. Process management
// (worktree creation, subprocess spawning, context assembly) lives
// in the host bridge layer.
package agents

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

// Schema is the DDL for the agent_activity table and its indices.
const Schema = `
CREATE TABLE IF NOT EXISTS agent_activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    task_key TEXT NOT NULL,
    agent TEXT NOT NULL,
    branch TEXT,
    worktree TEXT,
    status TEXT NOT NULL,
    pid INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER NOT NULL DEFAULT -1,
    monetary_cost REAL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    model TEXT,
    author TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    stopped_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_agent_activity_key ON agent_activity(key);
CREATE INDEX IF NOT EXISTS idx_agent_activity_task ON agent_activity(task_key);
CREATE INDEX IF NOT EXISTS idx_agent_activity_status ON agent_activity(status);
`

var (
	ErrNotFound    = errors.New("agent not found")
	ErrRunNotFound = errors.New("agent run not found")
	ErrRunning     = errors.New("agent already running for task")
)

// Agents provides agent configuration and run tracking. Config,
// prompts, and settings are plain files under dir. Run tracking
// uses the work database.
type Agents struct {
	dbFn func() *qwr.Manager
	db   *qwr.Manager
	dir  string // .llmd/agents/
	bus  *events.Bus
	once sync.Once
	err  error
}

// New creates an Agents instance. The db function returns the work
// database manager, opening it on demand if needed. dir is the base
// directory for agent files (typically .llmd/agents/).
func New(db func() *qwr.Manager, dir string, bus *events.Bus) *Agents {
	return &Agents{dbFn: db, dir: dir, bus: bus}
}

// ConfigPath returns the filesystem path for an agent's config.
func ConfigPath(name string) string {
	return filepath.Join(".llmd", "agents", name, "config.json")
}

// PromptPath returns the filesystem path for an agent's prompt template.
func PromptPath(name, role string) string {
	return filepath.Join(".llmd", "agents", name, role+".md")
}

// ensure opens the work database and creates the agent_activity table if needed.
func (a *Agents) ensure() error {
	a.once.Do(func() {
		a.db = a.dbFn()
		if a.db == nil {
			a.err = fmt.Errorf("work database not available")
			return
		}
		_, a.err = a.db.Query(Schema).Write()
	})
	return a.err
}

// hasRunning checks whether a task already has a running agent.
func (a *Agents) hasRunning(ctx context.Context, taskKey string) (bool, error) {
	row, err := a.db.Query(`
		SELECT COUNT(*) FROM agent_activity
		WHERE task_key = ? AND status = 'running'
	`, taskKey).WithContext(ctx).ReadRow()
	if err != nil {
		return false, err
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
