// checkpoint.go implements WAL checkpoint operations for SQLite.
//
// Separated because checkpointing is a maintenance operation with different
// usage patterns than normal reads/writes. Checkpoints are typically called
// on graceful shutdown or periodically during long-running processes.
//
// Design: We use TRUNCATE mode which fully flushes the WAL and removes the
// -wal/-shm files. This is appropriate for llmd's usage where clean shutdown
// is preferred over crash recovery speed. The trade-off is slightly slower
// recovery if the process crashes during a checkpoint.

package store

import (
	"context"
	"fmt"
)

// Checkpoint writes all WAL data back to the main database file and truncates
// the WAL. This removes the -wal and -shm files from the filesystem.
func (s *SQLiteStore) Checkpoint(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("WAL checkpoint: %w", err)
	}
	return nil
}
