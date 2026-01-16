// tools_search.go implements MCP tools for document search operations.
//
// Separated from tools_documents.go because search operations have different
// semantics - they return multiple documents and support query languages
// (FTS5 for search, regex for grep, sed expressions for transformations).
//
// Design: Search results are returned as JSON arrays for easy LLM parsing.
// The grep tool includes match context to help LLMs understand where patterns
// appear without fetching full documents.

package mcp

import (
	"bytes"
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/grep"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sed"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// searchDocuments handles llmd_search tool calls.
func (h *handlers) searchDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	pattern := getString(req, "pattern", "")

	paths, err := h.svc.Glob(ctx, pattern)

	log.Event("mcp:glob", "list").Author("mcp").Detail("pattern", pattern).Detail("count", len(paths)).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(paths)
}

// diffDocuments handles llmd_diff tool calls.
func (h *handlers) diffDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	opts := diff.Options{
		Path2:          getString(req, "path2", ""),
		Version1:       getInt(req, "version1", 0),
		Version2:       getInt(req, "version2", 0),
		IncludeDeleted: getBool(req, "include_deleted", false),
	}

	r, err := h.svc.Diff(ctx, path, opts)

	log.Event("mcp:diff", "diff").Author("mcp").Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]string{
		"old":  r.Old,
		"new":  r.New,
		"diff": r.Format(false),
	})
}

// grepDocuments handles llmd_grep tool calls.
func (h *handlers) grepDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

// sedDocument handles llmd_sed tool calls.
func (h *handlers) sedDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	expr, err := req.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError("expression is required"), nil //nolint:nilerr
	}

	opts := sed.Options{
		Author:  getString(req, "author", "mcp"),
		Message: getString(req, "message", ""),
	}

	var buf bytes.Buffer
	_, err = sed.Run(ctx, &buf, h.svc, path, expr, opts)

	log.Event("mcp:sed", "edit").Author(opts.Author).Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("edited %s", path)), nil
}
