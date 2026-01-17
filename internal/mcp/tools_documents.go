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

	"github.com/jpl-au/llmd/internal/edit"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// listDocuments handles llmd_list tool calls.
func (h *handlers) listDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prefix := getString(req, "prefix", "")
	includeDeleted := getBool(req, "include_deleted", false)
	deletedOnly := getBool(req, "deleted_only", false)
	tag := getString(req, "tag", "")

	var docs []store.Document
	var err error

	if tag != "" {
		docs, err = h.svc.ListByTag(ctx, prefix, tag, includeDeleted, deletedOnly, store.NewTagOptions())
	} else {
		docs, err = h.svc.List(ctx, prefix, includeDeleted, deletedOnly)
	}
	if err != nil {
		log.Event("mcp:list", "list").Author("mcp").Path(prefix).Detail("tag", tag).Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	log.Event("mcp:list", "list").Author("mcp").Path(prefix).Detail("count", len(docs)).Write(nil)

	result := make([]store.DocJSON, len(docs))
	for i := range docs {
		result[i] = docs[i].ToJSON(false)
	}

	return jsonResult(result)
}

// readDocumentTool handles llmd_read tool calls.
func (h *handlers) readDocumentTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	version := getInt(req, "version", 0)
	includeDeleted := getBool(req, "include_deleted", false)

	var doc *store.Document
	if version > 0 {
		doc, err = h.svc.Version(ctx, path, version)
	} else {
		// Use Resolve to support both paths and keys
		doc, _, err = h.svc.Resolve(ctx, path, includeDeleted)
	}

	v := 0
	if doc != nil {
		v = doc.Version
	}

	log.Event("mcp:read", "read").Author("mcp").Path(path).Version(v).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(doc.ToJSON(true))
}

// writeDocument handles llmd_write tool calls.
func (h *handlers) writeDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("content is required"), nil //nolint:nilerr
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil //nolint:nilerr
	}

	message := getString(req, "message", "")

	err = h.svc.Write(ctx, path, content, author, message)

	log.Event("mcp:write", "write").Author(author).Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("wrote %s", path)), nil
}

// deleteDocument handles llmd_delete tool calls.
// Supports both paths and 8-character keys via Resolve.
func (h *handlers) deleteDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	version := getInt(req, "version", 0)

	// For simple delete (no version), resolve as path or key
	if version == 0 {
		doc, isKey, err := h.svc.Resolve(ctx, path, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if isKey {
			// Resolved as key - delete that specific version
			err = h.svc.DeleteVersion(ctx, doc.Path, doc.Version)
			log.Event("mcp:delete_version", "delete").Author("mcp").Path(doc.Path).Version(doc.Version).Detail("key", path).Write(err)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("deleted %s (version %d, key %s)", doc.Path, doc.Version, path)), nil
		}
		// Resolved as path, update path and continue with normal delete below
		path = doc.Path
	}

	if version > 0 {
		err = h.svc.DeleteVersion(ctx, path, version)
		log.Event("mcp:delete_version", "delete").Author("mcp").Path(path).Version(version).Write(err)
	} else {
		err = h.svc.Delete(ctx, path)
		log.Event("mcp:delete", "delete").Author("mcp").Path(path).Write(err)
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
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	// Resolve as path or key (includeDeleted=true for restore)
	doc, isKey, err := h.svc.Resolve(ctx, path, true)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%q: %v", path, err)), nil
	}

	key := ""
	if isKey {
		key = path
	}
	path = doc.Path

	err = h.svc.Restore(ctx, path)

	logEvent := log.Event("mcp:restore", "restore").Author("mcp").Path(path)
	if key != "" {
		logEvent.Detail("key", key)
	}
	logEvent.Write(err)

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
	from, err := req.RequireString("from")
	if err != nil {
		return mcp.NewToolResultError("from is required"), nil //nolint:nilerr
	}

	to, err := req.RequireString("to")
	if err != nil {
		return mcp.NewToolResultError("to is required"), nil //nolint:nilerr
	}

	err = h.svc.Move(ctx, from, to)

	log.Event("mcp:move", "move").Author("mcp").Path(from).Detail("from", from).Detail("to", to).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("moved %s -> %s", from, to)), nil
}

// historyDocument handles llmd_history tool calls.
// Supports both paths and 8-character keys.
func (h *handlers) historyDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	limit := getInt(req, "limit", 0)
	includeDeleted := getBool(req, "include_deleted", false)

	// Resolve path or key to get the actual document path
	doc, _, err := h.svc.Resolve(ctx, path, includeDeleted)
	if err != nil {
		log.Event("mcp:history", "history").Author("mcp").Path(path).Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}
	resolvedPath := doc.Path

	docs, err := h.svc.History(ctx, resolvedPath, limit, includeDeleted)

	log.Event("mcp:history", "history").Author("mcp").Path(resolvedPath).Detail("count", len(docs)).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := make([]store.DocJSON, len(docs))
	for i := range docs {
		result[i] = docs[i].ToJSON(false)
	}

	return jsonResult(result)
}

// editDocument handles llmd_edit tool calls.
func (h *handlers) editDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	old, err := req.RequireString("old")
	if err != nil {
		return mcp.NewToolResultError("old is required"), nil //nolint:nilerr
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil //nolint:nilerr
	}

	opts := edit.Options{
		Old:     old,
		New:     getString(req, "new", ""),
		Author:  author,
		Message: getString(req, "message", ""),
	}

	err = h.svc.Edit(ctx, path, opts)

	log.Event("mcp:edit", "edit").Author(opts.Author).Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("edited %s", path)), nil
}
