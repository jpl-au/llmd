// move.go implements document rename and copy operations for the Service layer.
//
// Separated from write.go because move/copy involve coordinating database
// updates with filesystem operations. A move must update both the database
// path AND the filesystem file atomically from the user's perspective.
//
// Design: If filesystem sync fails after database move, we don't rollback
// the database - the database is the source of truth. The filesystem can be
// re-synced later. This prevents data loss at the cost of temporary inconsistency.

package document

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/store"
)

// Move renames a document.
func (s *Service) Move(ctx context.Context, src, dst string) error {
	opts := store.MoveOptions{
		MaxPath: s.maxPath,
	}

	if err := s.store.Move(ctx, src, dst, opts); err != nil {
		return fmt.Errorf("move %q to %q: %w", src, dst, err)
	}

	if err := s.syncMove(src, dst); err != nil {
		// Database move succeeded but filesystem sync failed.
		// Document exists at new path in DB, filesystem may be inconsistent.
		return fmt.Errorf("move %q to %q: database updated but filesystem sync failed: %w", src, dst, err)
	}

	// Fire event after successful move. Fetch document for version info.
	doc, err := s.store.Latest(ctx, dst, false)
	if err != nil {
		return fmt.Errorf("move %q to %q: fetch for event: %w", src, dst, err)
	}
	s.fireEvent(extension.DocumentWriteEvent{
		Path:    dst,
		Version: doc.Version,
		Author:  doc.Author,
		Message: fmt.Sprintf("moved from %s", src),
		Content: doc.Content,
	})
	return nil
}

// Copy duplicates a document to a new path. The copier parameter tracks
// who performed the copy operation for audit purposes.
func (s *Service) Copy(ctx context.Context, from, to, copier string) error {
	opts := store.CopyOptions{
		MaxPath: s.maxPath,
	}

	if err := s.store.Copy(ctx, from, to, copier, opts); err != nil {
		return fmt.Errorf("copy %q to %q: %w", from, to, err)
	}

	// Fetch the copied document for sync and event firing.
	doc, err := s.store.Latest(ctx, to, false)
	if err != nil {
		return fmt.Errorf("copy %q to %q: fetch: %w", from, to, err)
	}

	// Sync the copy to filesystem if enabled
	if s.syncFiles {
		if err := s.syncWrite(to, doc.Content); err != nil {
			return fmt.Errorf("sync %q: %w", to, err)
		}
	}

	// Fire event after successful copy.
	s.fireEvent(extension.DocumentWriteEvent{
		Path:    to,
		Version: doc.Version,
		Author:  copier,
		Message: fmt.Sprintf("copied from %s", from),
		Content: doc.Content,
	})
	return nil
}
