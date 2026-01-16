// Package vacuum handles permanent deletion of soft-deleted documents.
// This is the only way to reclaim storage; soft-deleted items remain
// until vacuum removes them, providing a recovery window.
package vacuum

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jpl-au/llmd/internal/progress"
	"github.com/jpl-au/llmd/internal/service"
)

// Options configures vacuum scope and safety checks.
type Options struct {
	OlderThan *time.Duration // Retain recent deletions for recovery
	Prefix    string         // Limit to specific path prefix
	DryRun    bool           // Preview without deleting
}

// Result reports what was deleted, enabling confirmation and logging.
type Result struct {
	Deleted int      // Count of removed documents
	Paths   []string // Affected paths (populated in dry-run mode)
}

// Run permanently removes soft-deleted documents. This operation is
// irreversible; use DryRun first to preview what will be deleted.
func Run(ctx context.Context, w io.Writer, svc service.Service, opts Options) (Result, error) {
	var result Result

	if opts.DryRun {
		return preview(ctx, w, svc, opts)
	}

	spin := progress.NewSpinner("Vacuuming")
	spin.Start()
	count, err := svc.Vacuum(ctx, opts.OlderThan, opts.Prefix)
	spin.Stop()

	if err != nil {
		return result, err
	}

	result.Deleted = int(count)
	if count == 0 {
		fmt.Fprintln(w, "No documents to vacuum")
	} else {
		fmt.Fprintf(w, "Vacuumed %d row(s)\n", count)
	}

	return result, nil
}

// preview simulates vacuum to let users verify before permanent deletion.
func preview(ctx context.Context, w io.Writer, svc service.Service, opts Options) (Result, error) {
	var result Result

	docs, err := svc.List(ctx, opts.Prefix, false, true) // deleted only
	if err != nil {
		return result, err
	}

	for _, doc := range docs {
		if doc.DeletedAt == nil {
			continue
		}

		// Skip recently deleted docs to give users time to recover
		if opts.OlderThan != nil {
			cutoff := time.Now().Add(-*opts.OlderThan).Unix()
			if *doc.DeletedAt >= cutoff {
				continue
			}
		}

		fmt.Fprintf(w, "Would delete: %s (deleted %s)\n",
			doc.Path,
			time.Unix(*doc.DeletedAt, 0).Format("2006-01-02 15:04"))
		result.Paths = append(result.Paths, doc.Path)
		result.Deleted++
	}

	if result.Deleted == 0 {
		fmt.Fprintln(w, "No documents to vacuum")
	} else {
		fmt.Fprintf(w, "\nWould delete %d document(s)\n", result.Deleted)
	}

	return result, nil
}
