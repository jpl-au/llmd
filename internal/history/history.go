// Package history provides document version history with optional diffs.
//
// Every write creates a new version, enabling full audit trails and rollback.
// The diff view shows what changed between versions, useful for code review
// and understanding document evolution over time.
package history

import (
	"context"
	"fmt"
	"io"

	"github.com/jpl-au/llmd/internal/format"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// Options configures a history operation.
type Options struct {
	Limit          int  // Maximum versions to return (0 = all)
	IncludeDeleted bool // Include deleted versions
	ShowDiff       bool // Show diffs between versions
	Colour         bool // Colourize diff output
}

// Result contains the outcome of a history operation.
type Result struct {
	Versions []store.Document
}

// Run retrieves document history and writes output to w.
// path can be a document path or a key - if key, shows history for that document.
func Run(ctx context.Context, w io.Writer, svc service.Service, path string, opts Options) (Result, error) {
	var result Result

	// Resolve path or key to get the actual document path
	doc, err := svc.Resolve(ctx, path, opts.IncludeDeleted)
	if err != nil {
		return result, err
	}
	path = doc.Path // Use resolved path for history

	docs, err := svc.History(ctx, path, opts.Limit, opts.IncludeDeleted)
	if err != nil {
		return result, err
	}

	if len(docs) == 0 {
		return result, fmt.Errorf("no history found for %s", path)
	}

	result.Versions = docs

	if opts.ShowDiff {
		err = format.HistoryDiff(w, docs, opts.Colour)
	} else {
		err = format.History(w, docs)
	}

	return result, err
}
