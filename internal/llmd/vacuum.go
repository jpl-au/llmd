// vacuum.go implements the explicit purge step for soft-deleted data.
//
// llmd uses a two-phase delete model: "rm" soft-deletes a document by
// setting deleted_at, keeping it recoverable via "restore". Vacuum is
// the second phase - it permanently removes soft-deleted documents,
// tags, and links, then runs SQLite VACUUM to reclaim disk space.
//
// Soft-deleting a document leaves its tags and links in place so that
// "restore" brings the document back intact. Purging a document is
// final, so vacuum also removes the tags and links it leaves orphaned.
//
// Vacuum also trims the message queue: every store change lands there
// as a message and nothing else removes one, so messages older than
// the store's queue retention (SetQueueRetention) go in the same pass.
//
// This is an explicit user action (not automatic) because accidental
// deletes should be recoverable until the user intentionally purges.

package llmd

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/clock"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

// VacuumOptions narrows a vacuum. Before purges only items deleted
// before that Unix-millisecond time (0 purges everything soft-deleted);
// DryRun reports what would be purged without changing anything.
type VacuumOptions struct {
	Before int64
	DryRun bool
}

// VacuumResult contains the results of a vacuum operation.
type VacuumResult struct {
	Documents int64    // Number of documents purged (all versions of each)
	Tags      int64    // Number of tags purged
	Links     int64    // Number of links purged
	Messages  int64    // Number of queue messages purged (older than the retention)
	Paths     []string // Paths of the purged documents, in path order
	DryRun    bool     // Nothing was changed; counts are what would be purged
}

// Total returns the total number of items purged.
func (r VacuumResult) Total() int64 {
	return r.Documents + r.Tags + r.Links + r.Messages
}

// Vacuum permanently deletes soft-deleted documents, the tags and links
// they orphan, and separately soft-deleted tags and links, removes
// queue messages older than the queue retention when one is set and
// work.db exists, then runs SQLite VACUUM to reclaim disk space. With
// DryRun it only reports. Outside a dry run this operation cannot be
// undone.
func (s *Store) Vacuum(ctx context.Context, opts VacuumOptions) (*VacuumResult, error) {
	result := VacuumResult{DryRun: opts.DryRun}

	paths, err := s.Documents.Purge(ctx, documents.PurgeOptions{Before: opts.Before, DryRun: opts.DryRun})
	if err != nil {
		return nil, fmt.Errorf("purging documents: %w", err)
	}
	result.Documents = int64(len(paths))
	result.Paths = paths

	n, err := s.Tags.Purge(ctx, tags.PurgeOptions{Before: opts.Before, DryRun: opts.DryRun})
	if err != nil {
		return nil, fmt.Errorf("purging tags: %w", err)
	}
	result.Tags = n

	n, err = s.Links.Purge(ctx, links.PurgeOptions{Before: opts.Before, DryRun: opts.DryRun})
	if err != nil {
		return nil, fmt.Errorf("purging links: %w", err)
	}
	result.Links = n

	if s.queueRetention > 0 && s.workOpen() {
		n, err = s.Messages.Purge(ctx, clock.Now()-s.queueRetention.Milliseconds(), opts.DryRun)
		if err != nil {
			return nil, fmt.Errorf("purging queue messages: %w", err)
		}
		result.Messages = n
	}

	if opts.DryRun {
		return &result, nil
	}

	// Run SQLite VACUUM to reclaim disk space.
	if _, err := s.db.Query("VACUUM").WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("vacuuming database: %w", err)
	}
	if work := s.openWork(); work != nil {
		if _, err := work.Query("VACUUM").WithContext(ctx).Exec(); err != nil {
			return nil, fmt.Errorf("vacuuming work database: %w", err)
		}
	}

	return &result, nil
}
