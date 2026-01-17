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
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query is required"), nil //nolint:nilerr
	}

	prefix := getString(req, "prefix", "")
	includeDeleted := getBool(req, "include_deleted", false)
	deletedOnly := getBool(req, "deleted_only", false)

	docs, err := h.svc.Search(ctx, query, prefix, includeDeleted, deletedOnly)

	log.Event("mcp:search", "search").Author("mcp").Path(prefix).Detail("query", query).Detail("count", len(docs)).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := make([]store.DocJSON, len(docs))
	for i := range docs {
		result[i] = docs[i].ToJSON(true)
	}

	return jsonResult(result)
}

// globDocuments handles llmd_glob tool calls.
func (h *handlers) globDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	pattern := getString(req, "pattern", "")

	paths, err := h.svc.Glob(ctx, pattern)

	log.Event("mcp:glob", "list").Author("mcp").Detail("pattern", pattern).Detail("count", len(paths)).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(paths)
}

// grepDocuments handles llmd_grep tool calls.
func (h *handlers) grepDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	pattern, err := req.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError("pattern is required"), nil //nolint:nilerr
	}

	opts := grep.Options{
		Path:          getString(req, "path", ""),
		IncludeAll:    getBool(req, "include_deleted", false),
		DeletedOnly:   getBool(req, "deleted_only", false),
		PathsOnly:     getBool(req, "paths_only", false),
		IgnoreCase:    getBool(req, "ignore_case", false),
		MaxLineLength: h.svc.MaxLineLength(),
	}

	var buf bytes.Buffer
	result, err := grep.Run(ctx, &buf, h.svc, pattern, opts)

	log.Event("mcp:grep", "search").Author("mcp").Path(opts.Path).Detail("pattern", pattern).Detail("count", len(result.Documents)).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	docs := make([]store.DocJSON, len(result.Documents))
	for i := range result.Documents {
		docs[i] = result.Documents[i].ToJSON(true)
	}

	return jsonResult(docs)
}
