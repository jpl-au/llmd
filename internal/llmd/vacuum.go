// vacuum.go implements the explicit purge step for soft-deleted data.
//
// llmd uses a two-phase delete model: "rm" soft-deletes a document by
// setting deleted_at, keeping it recoverable via "restore". Vacuum is
// the second phase — it permanently removes soft-deleted documents,
// tags, and links, then runs SQLite VACUUM to reclaim disk space.
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

	// Purge soft-deleted tags
	n, err = s.Tags.Purge(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging tags: %w", err)
	}
	result.Tags = n

	// Purge soft-deleted links
	n, err = s.Links.Purge(ctx)
	if err != nil {
		return nil, fmt.Errorf("purging links: %w", err)
	}
	result.Links = n

	// Run SQLite VACUUM to reclaim disk space
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return nil, fmt.Errorf("vacuuming database: %w", err)
	}

	return &result, nil
}
