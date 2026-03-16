// Package audits provides agent-to-agent and human-to-agent review threads.
//
// Audit records are insert-only — once written, a row is never updated
// (except for soft-delete via deleted_at). Thread status is derived from
// the most recent entry. The table is created lazily on first use, so
// stores that never use audits pay no schema cost.
//
// The package emits events via the event bus after mutations so
// cross-cutting concerns can react without coupling.
package audits

import (
	"errors"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

const schema = `
CREATE TABLE IF NOT EXISTS audits (
    id          TEXT PRIMARY KEY,
    target      TEXT     NOT NULL,
    target_type TEXT     NOT NULL,
    version     INTEGER,
    author      TEXT     NOT NULL,
    assignee    TEXT     NOT NULL DEFAULT '',
    status      TEXT     NOT NULL DEFAULT 'pending',
    content     TEXT     NOT NULL DEFAULT '',
    parent_id   TEXT,
    created_at  INTEGER  NOT NULL,
    deleted_at  INTEGER,
    FOREIGN KEY (parent_id) REFERENCES audits(id)
);

CREATE INDEX IF NOT EXISTS idx_audits_target     ON audits(target);
CREATE INDEX IF NOT EXISTS idx_audits_status     ON audits(status);
CREATE INDEX IF NOT EXISTS idx_audits_author     ON audits(author);
CREATE INDEX IF NOT EXISTS idx_audits_assignee   ON audits(assignee);
CREATE INDEX IF NOT EXISTS idx_audits_parent     ON audits(parent_id);
CREATE INDEX IF NOT EXISTS idx_audits_created_at ON audits(created_at);
`

var (
	ErrNotFound      = errors.New("audit not found")
	ErrMissingAuthor = errors.New("author is required")
	ErrMissingTarget = errors.New("target is required")
)

// Audits provides audit CRUD and thread status queries. The audits
// table is created lazily on first use via sync.Once.
type Audits struct {
	db   *qwr.Manager
	bus  *events.Bus
	once sync.Once
	err  error
}

// New creates an Audits instance.
func New(db *qwr.Manager, bus *events.Bus) *Audits {
	return &Audits{db: db, bus: bus}
}

// ensure creates the audits table if it does not exist.
func (a *Audits) ensure() error {
	a.once.Do(func() {
		_, a.err = a.db.Query(schema).Write()
	})
	return a.err
}
