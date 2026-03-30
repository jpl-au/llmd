// Package audits provides agent-to-agent and human-to-agent review threads.
//
// Audit records are insert-only - once written, a row is never updated
// (except for soft-delete via deleted_at). Thread status is derived from
// the most recent entry. The table is created lazily on first use, so
// stores that never use audits pay no schema cost.
//
// The package emits events via the event bus after mutations so
// cross-cutting concerns can react without coupling.
package audits

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

// Schema is the DDL for the audits table and its indices.
const Schema = `
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
// table is created when the work database is first opened.
type Audits struct {
	dbFn func() *qwr.Manager
	db   *qwr.Manager
	bus  *events.Bus
	once sync.Once
	err  error
}

// New creates an Audits instance. The db function returns the work
// database manager, opening it on demand if needed.
func New(db func() *qwr.Manager, bus *events.Bus) *Audits {
	return &Audits{dbFn: db, bus: bus}
}

// ensure opens the work database and creates the audits table if needed.
func (a *Audits) ensure() error {
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
