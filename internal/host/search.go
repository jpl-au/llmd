// This file implements search-related host functions.
//
// Search operations allow plugins to find documents by content. Three search
// modes are supported:
//   - Full-text: Searches document content using a search engine query syntax
//   - Regex: Searches document content using regular expressions
//   - Glob: Searches document paths using glob patterns
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd/search"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// SearchFullText performs a full-text search across all documents.
//
// The query uses the search engine's query syntax. Results are returned with
// snippets showing the matching text in context. Results are ordered by
// relevance score.
func (h *HostFuncs) SearchFullText(ctx context.Context, req *hostpb.SearchRequest) (*hostpb.SearchResults, error) {
	opts := search.Options{
		Path: req.Prefix,
	}

	docs, err := h.store.Search.FullText(ctx, req.Query, opts)
	if err != nil {
		return nil, err
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

	return &hostpb.SearchResults{Matches: matches}, nil
}

// SearchRegex performs a regular expression search across all documents.
//
// The pattern uses Go's regexp syntax. Results include the matching line,
// line number, and optionally context lines before and after. If IgnoreCase
// is true, the search is case-insensitive.
func (h *HostFuncs) SearchRegex(ctx context.Context, req *hostpb.GrepRequest) (*hostpb.GrepResults, error) {
	opts := search.Options{
		Path:       req.Path,
		IgnoreCase: req.IgnoreCase,
		Context:    int(req.ContextLines),
		Mode:       search.ModeContent,
	}

	result, err := h.store.Search.Regex(ctx, req.Pattern, opts)
	if err != nil {
		return nil, err
	}

	matches := make([]*hostpb.GrepMatch, len(result.Matches))
	for i, m := range result.Matches {
		var before, after []string
		lineIdx := -1
		for j, line := range m.Context {
			if line == m.Content {
				lineIdx = j
				break
			}
		}
		if lineIdx > 0 {
			before = m.Context[:lineIdx]
		}
		if lineIdx >= 0 && lineIdx < len(m.Context)-1 {
			after = m.Context[lineIdx+1:]
		}

		matches[i] = &hostpb.GrepMatch{
			Path:          m.Path,
			Line:          int32(m.Line),
			Content:       m.Content,
			ContextBefore: before,
			ContextAfter:  after,
		}
	}

	return &hostpb.GrepResults{Matches: matches}, nil
}

// SearchGlob finds documents matching a glob pattern.
//
// The pattern uses standard glob syntax (* matches any sequence, ? matches
// any single character). Returns a list of matching document paths.
func (h *HostFuncs) SearchGlob(ctx context.Context, req *hostpb.GlobRequest) (*hostpb.GlobResults, error) {
	paths, err := h.store.Search.Glob(ctx, req.Pattern)
	if err != nil {
		return nil, err
	}

	return &hostpb.GlobResults{Paths: paths}, nil
}
