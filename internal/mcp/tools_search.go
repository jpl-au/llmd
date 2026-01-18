// tools_search.go implements MCP tools for finding documents.
//
// These tools help LLMs locate content: FTS5 full-text search, glob pattern
// matching for paths, and regex grep for content. All return results as JSON
// arrays for easy parsing.

package mcp

import (
	"bytes"
	"context"

	"github.com/jpl-au/llmd/internal/grep"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// searchDocuments handles llmd_search tool calls.
func (h *handlers) searchDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query is required"), nil
	}

	prefix := getString(req, "prefix", "")
	includeDeleted := getBool(req, "include_deleted", false)
	deletedOnly := getBool(req, "deleted_only", false)
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:search", "search").Author(author).Path(prefix).Detail("query", query)
	defer func() { l.Write(err) }()

	docs, err := h.svc.Search(ctx, query, prefix, includeDeleted, deletedOnly)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", len(docs))

	searchResult := make([]store.DocJSON, len(docs))
	for i := range docs {
		searchResult[i] = docs[i].ToJSON(true)
	}

	return jsonResult(searchResult)
}

// globDocuments handles llmd_glob tool calls.
func (h *handlers) globDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	pattern := getString(req, "pattern", "")
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:glob", "list").Author(author).Detail("pattern", pattern)
	defer func() { l.Write(err) }()

	paths, err := h.svc.Glob(ctx, pattern)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", len(paths))

	return jsonResult(paths)
}

// grepDocuments handles llmd_grep tool calls.
func (h *handlers) grepDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	pattern, err := req.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError("pattern is required"), nil
	}

	opts := grep.Options{
		Path:          getString(req, "path", ""),
		IncludeAll:    getBool(req, "include_deleted", false),
		DeletedOnly:   getBool(req, "deleted_only", false),
		PathsOnly:     getBool(req, "paths_only", false),
		IgnoreCase:    getBool(req, "ignore_case", false),
		MaxLineLength: h.svc.MaxLineLength(),
	}
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:grep", "search").Author(author).Path(opts.Path).Detail("pattern", pattern)
	defer func() { l.Write(err) }()

	var buf bytes.Buffer
	grepResult, err := grep.Run(ctx, &buf, h.svc, pattern, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", len(grepResult.Documents))

	docs := make([]store.DocJSON, len(grepResult.Documents))
	for i := range grepResult.Documents {
		docs[i] = grepResult.Documents[i].ToJSON(true)
	}

	return jsonResult(docs)
}
