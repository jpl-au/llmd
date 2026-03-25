// Package agents manages agent configurations and execution records.
//
// Agent configurations and prompt templates are stored as documents
// under the agents/ path prefix:
//
//   - agents/<name>/config      operational config (JSON)
//   - agents/<name>/<role>      prompt template (markdown with Go templates)
//   - agents/default/<role>     fallback prompt templates
//
// This makes agent configuration portable, versioned, and manageable
// through the standard document interface. The agent CLI command
// provides the user-facing surface.
//
// Agent runs are tracked in a dedicated table so consumers can
// observe status and history.
//
// This package handles data operations only. Process management
// (worktree creation, subprocess spawning, context assembly) lives
// in the host bridge layer, following the same pattern as task git
// integration.
package agents

import (
	"context"
	"errors"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

const schema = `
CREATE TABLE IF NOT EXISTS agent_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    task_key TEXT NOT NULL,
    agent TEXT NOT NULL,
    branch TEXT,
    worktree TEXT,
    status TEXT NOT NULL,
    pid INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER NOT NULL DEFAULT -1,
    author TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    stopped_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_key ON agent_runs(key);
CREATE INDEX IF NOT EXISTS idx_agent_runs_task ON agent_runs(task_key);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
`

// Document path conventions for agent storage.
const (
	// PathPrefix is the root for all agent documents.
	PathPrefix = "agents/"

	// ConfigDoc is the document name for agent operational config.
	ConfigDoc = "config"
)

var (
	ErrNotFound    = errors.New("agent not found")
	ErrRunNotFound = errors.New("agent run not found")
	ErrRunning     = errors.New("agent already running for task")
)

// Agents provides agent configuration and run tracking.
type Agents struct {
	db   *qwr.Manager
	docs *documents.Documents
	bus  *events.Bus
	once sync.Once
	err  error
}

// New creates an Agents instance.
func New(db *qwr.Manager, docs *documents.Documents, bus *events.Bus) *Agents {
	return &Agents{db: db, docs: docs, bus: bus}
}

// ConfigPath returns the document path for an agent's config.
func ConfigPath(name string) string {
	return PathPrefix + name + "/" + ConfigDoc
}

// PromptPath returns the document path for an agent's prompt template.
func PromptPath(name, role string) string {
	return PathPrefix + name + "/" + role
}

// ensure creates the agent_runs table if it does not exist.
func (a *Agents) ensure() error {
	a.once.Do(func() {
		_, a.err = a.db.Query(schema).Write()
	})
	return a.err
}

// hasRunning checks whether a task already has a running agent.
func (a *Agents) hasRunning(ctx context.Context, taskKey string) (bool, error) {
	row, err := a.db.Query(`
		SELECT COUNT(*) FROM agent_runs
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
