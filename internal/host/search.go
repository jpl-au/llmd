// This file implements search-related host functions.
//
//   - Full-text: Searches document content using FTS5
//   - Glob: Matches document paths using glob patterns
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd/search"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// SearchFullText performs a full-text search across all documents.
//
// The query uses FTS5 query syntax. Results are returned with snippets
// showing the matching text in context. Results are ordered by relevance.
func (h *HostFuncs) SearchFullText(ctx context.Context, req *hostpb.SearchRequest) (*hostpb.SearchResponse, error) {
	if h.store == nil {
		return &hostpb.SearchResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := search.Options{
		Path: req.Prefix,
	}

	docs, err := h.store.Search.FullText(ctx, req.Query, opts)
	if err != nil {
		return &hostpb.SearchResponse{Error: err.Error()}, nil
	}

	matches := make([]*hostpb.SearchMatch, len(docs))
	for i, doc := range docs {
		snippet := doc.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}

		matches[i] = &hostpb.SearchMatch{
			Path:    doc.Path,
			Snippet: snippet,
			Score:   1.0,
		}
	}

	return &hostpb.SearchResponse{Success: true, Matches: matches}, nil
}

// SearchRegex performs a full-text search (FTS5) across all documents.
//
// The query is treated as an FTS5 query. Results include the matching
// document path and content.
func (h *HostFuncs) SearchRegex(ctx context.Context, req *hostpb.GrepRequest) (*hostpb.GrepResponse, error) {
	if h.store == nil {
		return &hostpb.GrepResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := search.Options{
		Path: req.Path,
	}

	docs, err := h.store.Search.FullText(ctx, req.Pattern, opts)
	if err != nil {
		return &hostpb.GrepResponse{Error: err.Error()}, nil
	}

	matches := make([]*hostpb.GrepMatch, len(docs))
	for i, doc := range docs {
		matches[i] = &hostpb.GrepMatch{
			Path:    doc.Path,
			Line:    1,
			Content: doc.Content,
		}
	}

	return &hostpb.GrepResponse{Success: true, Matches: matches}, nil
}

// SearchGlob finds documents matching a glob pattern.
//
// The pattern uses standard glob syntax (* matches any sequence, ? matches
// any single character). Returns a list of matching document paths.
func (h *HostFuncs) SearchGlob(ctx context.Context, req *hostpb.GlobRequest) (*hostpb.GlobResponse, error) {
	if h.store == nil {
		return &hostpb.GlobResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	paths, err := h.store.Search.Glob(ctx, req.Pattern)
	if err != nil {
		return &hostpb.GlobResponse{Error: err.Error()}, nil
	}

	return &hostpb.GlobResponse{Success: true, Paths: paths}, nil
}
