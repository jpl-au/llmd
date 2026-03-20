// Package messages provides the message queue implementation.
//
// Messages and acks are stored in two insert-only tables, created
// lazily on first use. The queue is strictly ordered - consumers
// must process messages front to back and acknowledge each one
// before moving to the next.
package messages

import (
	"errors"
	"sync"

	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/qwr"
)

const schema = `
CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT NOT NULL UNIQUE,
    source_key  TEXT,
    type        TEXT NOT NULL,
    payload     TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL,
    source      TEXT NOT NULL,
    assigned_to TEXT,
    created_at  INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_source_key ON messages(source_key) WHERE source_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_assigned ON messages(assigned_to) WHERE assigned_to IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);

CREATE TABLE IF NOT EXISTS message_acks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_key TEXT NOT NULL,
    author      TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    UNIQUE(message_key, author)
);

CREATE INDEX IF NOT EXISTS idx_message_acks_lookup ON message_acks(author, message_key);
`

var (
	ErrNotFound       = errors.New("message not found")
	ErrOrderViolation = errors.New("must acknowledge oldest message first")
)

// Messages provides queue operations backed by SQLite.
type Messages struct {
	db   *qwr.Manager
	bus  *events.Bus
	once sync.Once
	err  error
}

// New creates a Messages instance. Tables are created lazily on first use.
func New(db *qwr.Manager, bus *events.Bus) *Messages {
	return &Messages{db: db, bus: bus}
}

// ensure creates the messages and message_acks tables if they do not exist.
func (m *Messages) ensure() error {
	m.once.Do(func() {
		_, m.err = m.db.Query(schema).Write()
	})
	return m.err
}
