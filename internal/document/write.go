// write.go implements document creation, update, and deletion operations.
//
// Separated from service.go to isolate mutating operations. Write operations
// handle both database persistence and filesystem synchronisation, firing
// extension events only after both succeed.
//
// Design: Filesystem sync happens after database write but before event firing.
// This ensures extensions are only notified of fully-committed changes. If sync
// fails, the database change is not rolled back - this is intentional because
// the database is the source of truth; the filesystem is just a mirror.

package document

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/jpl-au/llmd/internal/sync"
)

// Write creates or updates a document.
func (s *Service) Write(ctx context.Context, path, content, author, message string) error {
	opts := store.WriteOptions{
		Author:     author,
		Message:    message,
		MaxPath:    s.maxPath,
		MaxContent: s.maxContent,
	}
	if opts.Author == "" {
		opts.Author = DefaultAuthor
	}

	if err := s.store.Write(ctx, path, content, opts); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}

	// Sync to filesystem before firing event.
	// This ensures extensions are only notified after the full operation succeeds.
	if err := s.syncWrite(path, content); err != nil {
		return fmt.Errorf("sync %q: %w", path, err)
	}

	// Fetch the written document for event firing.
	// Error is unlikely after successful write, but check to avoid nil panic.
	doc, err := s.store.Latest(ctx, path, false)
	if err != nil {
		return fmt.Errorf("retrieving written doc %q: %w", path, err)
	}
	s.fireEvent(extension.DocumentWriteEvent{
		Path:    path,
		Version: doc.Version,
		Author:  author,
		Message: message,
		Content: content,
	})

	return nil
}

// Delete soft-deletes a document.
func (s *Service) Delete(ctx context.Context, path string) error {
	opts := store.DeleteOptions{
		MaxPath: s.maxPath,
	}

	if err := s.store.Delete(ctx, path, opts); err != nil {
		return fmt.Errorf("delete %q: %w", path, err)
	}

	// Sync to filesystem before firing event.
	if err := s.syncRemove(path); err != nil {
		return fmt.Errorf("sync remove %q: %w", path, err)
	}

	s.fireEvent(extension.DocumentDeleteEvent{Path: path})
	return nil
}

// DeleteVersion soft-deletes a specific version of a document.
// Other versions remain accessible. If the deleted version was the latest,
// the filesystem is updated to reflect the new latest version.
func (s *Service) DeleteVersion(ctx context.Context, path string, version int) error {
	opts := store.DeleteVersionOptions{
		MaxPath: s.maxPath,
	}

	if err := s.store.DeleteVersion(ctx, path, version, opts); err != nil {
		return fmt.Errorf("delete version %d of %q: %w", version, path, err)
	}

	// Check if any versions remain to determine filesystem sync behaviour.
	// Latest() with includeDeleted=false will return ErrNotFound if all versions are deleted.
	doc, err := s.store.Latest(ctx, path, false)
	if errors.Is(err, store.ErrNotFound) {
		// All versions deleted - remove from filesystem
		if err := s.syncRemove(path); err != nil {
			return fmt.Errorf("sync remove %q: %w", path, err)
		}
	} else if err != nil {
		return fmt.Errorf("checking remaining versions for %q: %w", path, err)
	} else {
		// Versions remain - sync the new latest to filesystem
		if err := s.syncWrite(path, doc.Content); err != nil {
			return fmt.Errorf("sync %q: %w", path, err)
		}
	}

	s.fireEvent(extension.DocumentDeleteEvent{Path: path, Version: version})
	return nil
}

// Restore restores a soft-deleted document.
func (s *Service) Restore(ctx context.Context, path string) error {
	opts := store.RestoreOptions{
		MaxPath: s.maxPath,
	}

	if err := s.store.Restore(ctx, path, opts); err != nil {
		return fmt.Errorf("restore %q: %w", path, err)
	}

	doc, err := s.store.Latest(ctx, path, false)
	if err != nil {
		return fmt.Errorf("restore %q: fetch latest: %w", path, err)
	}

	s.fireEvent(extension.DocumentRestoreEvent{Path: path, Version: doc.Version})

	if err := s.syncWrite(path, doc.Content); err != nil {
		return fmt.Errorf("sync %q: %w", path, err)
	}
	return nil
}

// syncWrite writes a document to the filesystem mirror if sync is enabled.
// The filesystem is a mirror of the database, not the source of truth.
func (s *Service) syncWrite(path, content string) error {
	if !s.syncFiles {
		return nil
	}
	return sync.WriteFile(s.filesDir, path, content)
}

// syncRemove deletes a file from the filesystem mirror if sync is enabled.
func (s *Service) syncRemove(path string) error {
	if !s.syncFiles {
		return nil
	}
	return sync.RemoveFile(s.filesDir, path)
}

// syncMove renames a file in the filesystem mirror if sync is enabled.
func (s *Service) syncMove(src, dst string) error {
	if !s.syncFiles {
		return nil
	}
	return sync.MoveFile(s.filesDir, src, dst)
}
