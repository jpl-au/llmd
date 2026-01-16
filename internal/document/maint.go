// maint.go implements database maintenance operations for the Service layer.
//
// Separated because maintenance operations (vacuum, checkpoint) have different
// usage patterns and risk profiles than normal CRUD operations. These are
// typically run on schedules or before backups, not during normal usage.
//
// Design: Vacuum is the only way to permanently delete data. This separation
// makes the destructive nature explicit - you have to consciously call a
// maintenance operation to lose data permanently.

package document

import (
	"context"
	"time"

	"github.com/jpl-au/llmd/internal/path"
)

// Vacuum permanently removes soft-deleted documents.
func (s *Service) Vacuum(ctx context.Context, olderThan *time.Duration, prefix string) (int64, error) {
	if prefix != "" {
		var err error
		prefix, err = path.Normalise(prefix)
		if err != nil {
			return 0, err
		}
	}
	return s.store.Vacuum(ctx, olderThan, prefix)
}

// Checkpoint flushes the WAL to the main database file. Removes the -wal and
// -shm files from the filesystem, useful before backup operations or when
// preparing the database for distribution.
func (s *Service) Checkpoint(ctx context.Context) error {
	return s.store.Checkpoint(ctx)
}
