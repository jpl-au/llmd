// Package find provides FTS5 full-text search for documents.
//
// This wraps the service.Search method with output formatting, separating
// the search logic from presentation. FTS5 queries support prefix matching
// (word*) and boolean operators, making them ideal for natural language queries.
package find

import (
	"context"
	"io"

	"github.com/jpl-au/llmd/internal/format"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// Options configures a search operation.
type Options struct {
	Prefix      string // Scope search to path prefix
	IncludeAll  bool   // Include deleted documents
	DeletedOnly bool   // Search only deleted documents
	PathsOnly   bool   // Only output paths
}

// Result contains the outcome of a search operation.
type Result struct {
	Documents []store.Document
}

// Run searches documents and writes output to w.
func Run(ctx context.Context, w io.Writer, svc service.Service, query string, opts Options) (Result, error) {
	var result Result

	docs, err := svc.Search(ctx, query, opts.Prefix, opts.IncludeAll, opts.DeletedOnly)
	if err != nil {
		return result, err
	}

	result.Documents = docs

	if opts.PathsOnly {
		err = format.Paths(w, docs)
	} else {
		err = format.SearchResults(w, docs, query)
	}

	return result, err
}
