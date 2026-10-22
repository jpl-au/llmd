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
// This is an explicit user action (not automatic) because accidental
// deletes should be recoverable until the user intentionally purges.

package llmd

import (
	"context"
	"fmt"
)

// VacuumResult contains the results of a vacuum operation.
type VacuumResult struct {
	Documents int64 // Number of documents purged
	Tags      int64 // Number of tags purged
	Links     int64 // Number of links purged
}

// Total returns the total number of rows purged.
func (r VacuumResult) Total() int64 {
	return r.Documents + r.Tags + r.Links
}

// Vacuum permanently deletes all soft-deleted data and reclaims disk space.
// This operation cannot be undone.
func (s *Store) Vacuum(ctx context.Context) (*VacuumResult, error) {
	var result VacuumResult

	// Purge soft-deleted documents
	n, err := s.Documents.Purge(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging documents: %w", err)
	}
	result.Documents = n

	// Purge soft-deleted tags, then tags orphaned by the document purge.
	n, err = s.Tags.Purge(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging tags: %w", err)
	}
	result.Tags = n
	n, err = s.Tags.PurgeOrphans(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging orphaned tags: %w", err)
	}
	result.Tags += n

	// Purge soft-deleted links, then links orphaned by the document purge.
	n, err = s.Links.Purge(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging links: %w", err)
	}
	result.Links = n
	n, err = s.Links.PurgeOrphans(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging orphaned links: %w", err)
	}
	result.Links += n

	// Run SQLite VACUUM to reclaim disk space.
	if _, err := s.db.Query("VACUUM").WithContext(ctx).Exec(); err != nil {
		return nil, fmt.Errorf("vacuuming database: %w", err)
	}
	if s.work != nil {
		if _, err := s.work.Query("VACUUM").WithContext(ctx).Exec(); err != nil {
			return nil, fmt.Errorf("vacuuming work database: %w", err)
		}
	}

	return &result, nil
}
