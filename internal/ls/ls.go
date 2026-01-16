// Package ls provides document listing with sorting and filtering.
//
// There are two code paths for listing documents:
//
//  1. Standard listing uses List which loads full document content. This is
//     needed for tree view and simple listings where we already have the data.
//
//  2. Long format uses ListMeta which fetches only metadata from the database.
//     This avoids loading document content into memory just to display a table
//     of metadata. ListMeta also provides document size via SQL length() which
//     is more efficient than loading content and measuring it in Go.
//
// Both paths support the same filtering and sorting options. The split exists
// purely for efficiency when displaying the long format with size information.
package ls

import (
	"context"
	"io"
	"sort"
	"time"

	"github.com/jpl-au/llmd/internal/format"
	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// SortField specifies how to sort results.
// Time-based sorting is particularly useful for LLMs to answer "what changed recently?"
// and focus attention on active documents rather than stale ones.
type SortField string

const (
	SortNone SortField = ""
	SortName SortField = "name"
	SortTime SortField = "time" // newest first by default (most relevant for "what's new?")
)

// Options configures a list operation.
type Options struct {
	Prefix      string    // Filter by path prefix
	IncludeAll  bool      // Include deleted documents
	DeletedOnly bool      // Show only deleted documents
	Tree        bool      // Display as tree
	Long        bool      // Long format with metadata
	Tag         string    // Filter by tag
	Sort        SortField // Sort field (name, time)
	Reverse     bool      // Reverse sort order
}

// Result contains the outcome of a list operation.
//
// Only one of Documents or Metas will be populated, never both. Standard
// listings populate Documents. Long format listings populate Metas because
// they use the more efficient ListMeta query which avoids loading content.
type Result struct {
	Documents []store.Document
	Metas     []store.DocumentMeta
}

// Count returns the number of documents in the result.
func (r Result) Count() int {
	if len(r.Metas) > 0 {
		return len(r.Metas)
	}
	return len(r.Documents)
}

// MetaJSON is the API-friendly representation of DocumentMeta.
type MetaJSON struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Version   int    `json:"version"`
	Author    string `json:"author"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
	Size      int64  `json:"size"`
	Deleted   bool   `json:"deleted,omitempty"`
}

// ToJSON converts the result to JSON-serializable format.
func (r Result) ToJSON() any {
	if len(r.Metas) > 0 {
		out := make([]MetaJSON, len(r.Metas))
		for i, m := range r.Metas {
			out[i] = MetaJSON{
				Key:       m.Key,
				Path:      m.Path,
				Version:   m.Version,
				Author:    m.Author,
				Message:   m.Message,
				CreatedAt: time.Unix(m.CreatedAt, 0).UTC().Format(time.RFC3339),
				Size:      m.Size,
				Deleted:   m.DeletedAt != nil,
			}
		}
		return out
	}
	out := make([]store.DocJSON, len(r.Documents))
	for i := range r.Documents {
		out[i] = r.Documents[i].ToJSON(false)
	}
	return out
}

// Run lists documents and writes formatted output to w.
//
// Long format is handled separately by runLong for efficiency. All other
// formats use this function which loads full documents via List.
func Run(ctx context.Context, w io.Writer, svc service.Service, opts Options) (Result, error) {
	var result Result

	if opts.Long {
		return runLong(ctx, w, svc, opts)
	}

	docs, err := svc.List(ctx, opts.Prefix, opts.IncludeAll, opts.DeletedOnly)
	if err != nil {
		return result, err
	}

	if opts.Tag != "" {
		tagged, err := svc.PathsWithTag(ctx, opts.Tag, store.NewTagOptions())
		if err != nil {
			return result, err
		}

		// Create set for O(1) lookup
		allowed := make(map[string]bool, len(tagged))
		for _, p := range tagged {
			allowed[p] = true
		}

		// Filter docs
		var filtered []store.Document
		for _, d := range docs {
			if allowed[d.Path] {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}

	// Sort results. Name sorting is alphabetical by path. Time sorting shows
	// newest first by default, which is most useful for "what changed recently?"
	// questions. When timestamps match, we use path as a tie-breaker to ensure
	// consistent ordering across runs.
	switch opts.Sort {
	case SortName:
		sort.Slice(docs, func(i, j int) bool {
			if opts.Reverse {
				return docs[i].Path > docs[j].Path
			}
			return docs[i].Path < docs[j].Path
		})
	case SortTime:
		sort.Slice(docs, func(i, j int) bool {
			if docs[i].CreatedAt == docs[j].CreatedAt {
				if opts.Reverse {
					return docs[i].Path > docs[j].Path
				}
				return docs[i].Path < docs[j].Path
			}
			if opts.Reverse {
				return docs[i].CreatedAt < docs[j].CreatedAt
			}
			return docs[i].CreatedAt > docs[j].CreatedAt
		})
	}

	result.Documents = docs

	if opts.Tree {
		err = format.Tree(w, docs)
	} else {
		err = format.List(w, docs)
	}

	return result, err
}

// runLong handles long format listing.
//
// This is a separate function because long format needs document size, which
// ListMeta provides efficiently via SQL length(). Using the standard List
// would load all document content into memory just to measure its length.
//
// The filtering and sorting logic mirrors Run but operates on DocumentMeta
// instead of Document.
func runLong(ctx context.Context, w io.Writer, svc service.Service, opts Options) (Result, error) {
	var result Result

	// ListMeta has a simpler signature than List. It takes a single boolean
	// for whether to include deleted documents, so we combine the two flags.
	includeDeleted := opts.IncludeAll || opts.DeletedOnly
	metas, err := svc.ListMeta(ctx, opts.Prefix, includeDeleted)
	if err != nil {
		return result, err
	}

	// Filter to deleted only if requested
	if opts.DeletedOnly {
		var filtered []store.DocumentMeta
		for _, m := range metas {
			if m.DeletedAt != nil {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}

	// Filter by tag if specified
	if opts.Tag != "" {
		tagged, err := svc.PathsWithTag(ctx, opts.Tag, store.NewTagOptions())
		if err != nil {
			return result, err
		}

		allowed := make(map[string]bool, len(tagged))
		for _, p := range tagged {
			allowed[p] = true
		}

		var filtered []store.DocumentMeta
		for _, m := range metas {
			if allowed[m.Path] {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}

	// Sort results. Name sorting is alphabetical by path. Time sorting shows
	// newest first by default, which is most useful for "what changed recently?"
	// questions. When timestamps match, we use path as a tie-breaker to ensure
	// consistent ordering across runs.
	switch opts.Sort {
	case SortName:
		sort.Slice(metas, func(i, j int) bool {
			if opts.Reverse {
				return metas[i].Path > metas[j].Path
			}
			return metas[i].Path < metas[j].Path
		})
	case SortTime:
		sort.Slice(metas, func(i, j int) bool {
			if metas[i].CreatedAt == metas[j].CreatedAt {
				if opts.Reverse {
					return metas[i].Path > metas[j].Path
				}
				return metas[i].Path < metas[j].Path
			}
			if opts.Reverse {
				return metas[i].CreatedAt < metas[j].CreatedAt
			}
			return metas[i].CreatedAt > metas[j].CreatedAt
		})
	}

	result.Metas = metas
	err = format.LongMeta(w, metas)
	return result, err
}
