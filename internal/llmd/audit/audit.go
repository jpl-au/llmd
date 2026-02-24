// Package audit provides a general-purpose audit log.
//
// The history table is created lazily on first use — stores that never
// need auditing (document-only repos) never have this table.
//
// Every task state change writes a row here: who changed what, when,
// and what the old and new values were. The table is not coupled to
// tasks — future features can log events here too.
package audit

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

const schema = `
CREATE TABLE IF NOT EXISTS history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    subject TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT
);

CREATE INDEX IF NOT EXISTS idx_history_subject ON history(subject);
CREATE INDEX IF NOT EXISTS idx_history_action ON history(action);
CREATE INDEX IF NOT EXISTS idx_history_timestamp ON history(timestamp);
`

// Log provides audit logging operations.
type Log struct {
	db   *sql.DB
	once sync.Once
	err  error
}

// New creates a new Log instance.
func New(db *sql.DB) *Log {
	return &Log{db: db}
}

// ensure creates the history table if it does not exist.
func (l *Log) ensure() error {
	l.once.Do(func() {
		_, l.err = l.db.Exec(schema)
	})
	return l.err
}

// Record writes an audit event.
func (l *Log) Record(ctx context.Context, actor, action, subject, oldValue, newValue string) error {
	if err := l.ensure(); err != nil {
		return fmt.Errorf("audit: creating table: %w", err)
	}

	now := time.Now().UnixMilli()

	var oldV, newV sql.NullString
	if oldValue != "" {
		oldV = sql.NullString{String: oldValue, Valid: true}
	}
	if newValue != "" {
		newV = sql.NullString{String: newValue, Valid: true}
	}

	_, err := l.db.ExecContext(ctx, `
		INSERT INTO history (timestamp, actor, action, subject, old_value, new_value)
		VALUES (?, ?, ?, ?, ?, ?)
	`, now, actor, action, subject, oldV, newV)

	if err != nil {
		return fmt.Errorf("audit: recording event: %w", err)
	}
	return nil
}
