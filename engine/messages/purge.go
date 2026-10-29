package messages

import (
	"context"
	"database/sql"
	"fmt"
)

// Purge deletes the messages created before the Unix-millisecond time
// before, together with the acknowledgements recorded against them,
// and returns how many messages went. With dryRun it only counts.
//
// Age is the only test. A message a consumer has not acknowledged goes
// too: the queue keeps a bounded span of history, not every message
// until each possible reader has seen it, otherwise a consumer name
// that never acknowledges would pin the table forever.
func (m *Messages) Purge(ctx context.Context, before int64, dryRun bool) (int64, error) {
	if err := m.ensure(); err != nil {
		return 0, fmt.Errorf("messages: %w", err)
	}

	if dryRun {
		var n int64
		row, err := m.db.Query("SELECT COUNT(*) FROM messages WHERE created_at < ?", before).WithContext(ctx).ReadRow()
		if err != nil {
			return 0, fmt.Errorf("counting purgeable messages: %w", err)
		}
		if err := row.Scan(&n); err != nil {
			return 0, fmt.Errorf("counting purgeable messages: %w", err)
		}
		return n, nil
	}

	result, err := m.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM message_acks
			WHERE message_key IN (SELECT key FROM messages WHERE created_at < ?)
		`, before); err != nil {
			return nil, fmt.Errorf("purging acknowledgements: %w", err)
		}
		res, err := tx.ExecContext(ctx, "DELETE FROM messages WHERE created_at < ?", before)
		if err != nil {
			return nil, fmt.Errorf("purging messages: %w", err)
		}
		return res.RowsAffected()
	}).WithContext(ctx).Exec()
	if err != nil {
		return 0, err
	}
	return result.Value.(int64), nil
}
