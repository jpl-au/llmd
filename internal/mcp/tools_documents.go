// tools_documents.go implements MCP tools for document CRUD operations.
//
// Separated from server.go to isolate document-specific tool implementations.
// These tools mirror the CLI commands (list, cat, write, edit, rm, restore)
// but return structured JSON for LLM consumption.
//
// Design: Author is required for all write operations to ensure proper
// attribution in the audit trail, distinguishing between different agents
// and human CLI usage. Errors return tool results (not Go errors) to give
// LLMs actionable feedback.

package mcp

import (
	"context"
	"fmt"
	"io"

	"github.com/jpl-au/llmd/internal/edit"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/ls"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// listDocuments handles llmd_list tool calls.
// Uses internal/ls.Run() for consistency with CLI, including sort support.
func (h *handlers) listDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	opts := ls.Options{
		Prefix:      getString(req, "prefix", ""),
		IncludeAll:  getBool(req, "include_deleted", false),
		DeletedOnly: getBool(req, "deleted_only", false),
		Tag:         getString(req, "tag", ""),
		Reverse:     getBool(req, "reverse", false),
	}

	// Validate and set sort field
	sortBy := getString(req, "sort", "")
	if sortBy != "" && sortBy != "name" && sortBy != "time" {
		return mcp.NewToolResultError(fmt.Sprintf("invalid sort field %q: must be 'name' or 'time'", sortBy)), nil
	}
	opts.Sort = ls.SortField(sortBy)

	var err error
	author := getString(req, "author", "mcp")
	l := log.Event("mcp:list", "list").Author(author).Path(opts.Prefix).Detail("tag", opts.Tag).Detail("sort", sortBy)
	defer func() { l.Write(err) }()

	// Run ls with io.Discard - we only need the result, not text output
	lsResult, err := ls.Run(ctx, io.Discard, h.svc, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", lsResult.Count())

	return jsonResult(lsResult.ToJSON())
}

// readDocumentTool handles llmd_read tool calls.
func (h *handlers) readDocumentTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	version := getInt(req, "version", 0)
	includeDeleted := getBool(req, "include_deleted", false)
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:read", "read").Author(author).Path(path)
	defer func() { l.Write(err) }()

	var doc *store.Document
	if version > 0 {
		doc, err = h.svc.Version(ctx, path, version)
	} else {
		// Use Resolve to support both paths and keys
		doc, _, err = h.svc.Resolve(ctx, path, includeDeleted)
	}

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.ResultVersion(doc.Version)
	return jsonResult(doc.ToJSON(true))
}

// writeDocument handles llmd_write tool calls.
func (h *handlers) writeDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("content is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	message := getString(req, "message", "")

	l := log.Event("mcp:write", "write").Author(author).Path(path)
	defer func() { l.Write(err) }()

	err = h.svc.Write(ctx, path, content, author, message)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("wrote %s", path)), nil
}

// deleteDocument handles llmd_delete tool calls.
// Supports both paths and 8-character keys via Resolve.
func (h *handlers) deleteDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	version := getInt(req, "version", 0)
	inputPath := path // preserve original input for logging

	l := log.Event("mcp:delete", "delete").Author(author).Path(inputPath)
	defer func() { l.Write(err) }()

	// For simple delete (no version), resolve as path or key
	if version == 0 {
		var doc *store.Document
		var isKey bool
		doc, isKey, err = h.svc.Resolve(ctx, path, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if isKey {
			// Resolved as key - delete that specific version
			l.Resolved(doc.Path).Version(doc.Version).Detail("key", path)
			err = h.svc.DeleteVersion(ctx, doc.Path, doc.Version)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("deleted %s (version %d, key %s)", doc.Path, doc.Version, path)), nil
		}
		// Resolved as path, update path and continue with normal delete below
		path = doc.Path
	}

	if path != inputPath {
		l.Resolved(path)
	}
	if version > 0 {
		l.Version(version)
		err = h.svc.DeleteVersion(ctx, path, version)
	} else {
		err = h.svc.Delete(ctx, path)
	}

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if version > 0 {
		return mcp.NewToolResultText(fmt.Sprintf("deleted %s (version %d)", path, version)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("deleted %s", path)), nil
}

// restoreDocument handles llmd_restore tool calls.
// Supports both paths and 8-character keys via Resolve.
func (h *handlers) restoreDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	inputPath := path
	l := log.Event("mcp:restore", "restore").Author(author).Path(inputPath)
	defer func() { l.Write(err) }()

	// Resolve as path or key (includeDeleted=true for restore)
	doc, isKey, err := h.svc.Resolve(ctx, path, true)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%q: %v", path, err)), nil
	}

	key := ""
	if isKey {
		key = path
		l.Detail("key", key)
	}
	path = doc.Path
	if path != inputPath {
		l.Resolved(path)
	}

	err = h.svc.Restore(ctx, path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if key != "" {
		return mcp.NewToolResultText(fmt.Sprintf("restored %s (from key %s)", path, key)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("restored %s", path)), nil
}

// moveDocument handles llmd_move tool calls.
func (h *handlers) moveDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	from, err := req.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError("from is required"), nil
	}

	to, err := req.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError("to is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	l := log.Event("mcp:move", "move").Author(author).Path(from).Detail("from", from).Detail("to", to)
	defer func() { l.Write(err) }()

	err = h.svc.Move(ctx, from, to)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("moved %s -> %s", from, to)), nil
}

// historyDocument handles llmd_history tool calls.
// Supports both paths and 8-character keys.
func (h *handlers) historyDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	limit := getInt(req, "limit", 0)
	includeDeleted := getBool(req, "include_deleted", false)
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:history", "history").Author(author).Path(path)
	defer func() { l.Write(err) }()

	// Resolve path or key to get the actual document path
	doc, _, err := h.svc.Resolve(ctx, path, includeDeleted)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resolvedPath := doc.Path

	docs, err := h.svc.History(ctx, resolvedPath, limit, includeDeleted)
	if err != nil {
		l.Resolved(resolvedPath)
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Resolved(resolvedPath).Detail("count", len(docs))

	historyResult := make([]store.DocJSON, len(docs))
	for i := range docs {
		historyResult[i] = docs[i].ToJSON(false)
	}

	return jsonResult(historyResult)
}

// editDocument handles llmd_edit tool calls.
func (h *handlers) editDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	old, err := req.RequireString("old")
	if err != nil {
		return mcp.NewToolResultError("old is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	opts := edit.Options{
		Old:     old,
		New:     getString(req, "new", ""),
		Author:  author,
		Message: getString(req, "message", ""),
	}

	l := log.Event("mcp:edit", "edit").Author(opts.Author).Path(path)
	defer func() { l.Write(err) }()

	err = h.svc.Edit(ctx, path, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("edited %s", path)), nil
}
