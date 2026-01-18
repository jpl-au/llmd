// tools_documents.go implements MCP tools for document CRUD operations.
//
// Separated from server.go to isolate document-specific tool implementations
// and keep file sizes manageable. These tools mirror the CLI commands (list,
// cat, write, edit, rm, restore) but return structured JSON for LLM consumption
// rather than human-readable text.
//
// Design principles:
//
//  1. Author attribution: All write operations require an author parameter to
//     maintain a complete audit trail. This distinguishes between different LLM
//     agents (claude-code, cursor, etc.) and human CLI usage, which is critical
//     for debugging and understanding document history.
//
//  2. Error handling: Errors return MCP tool error results rather than Go errors.
//     This ensures the LLM receives actionable feedback it can parse and potentially
//     retry, rather than causing the entire tool call to fail at the protocol level.
//
//  3. Path resolution: Tools accept both document paths and 8-character keys,
//     using svc.Resolve() to handle the ambiguity. This flexibility lets LLMs
//     reference documents however is most convenient from their context.

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
//
// We delegate to internal/ls.Run() rather than calling store methods directly
// to ensure the MCP server behaves identically to the CLI. This is important
// because users may switch between CLI and MCP usage, and inconsistent behaviour
// (different sort orders, different filtering logic) would be confusing. The
// io.Discard writer discards text output since we only need the structured result.
//
// The author parameter defaults to "mcp" for logging purposes when not provided,
// which is acceptable for read-only operations but would be rejected for writes.
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
//
// Accepts an array of paths to support batch reads, reducing the number of
// round-trips between the LLM and MCP server when the LLM needs multiple
// documents. This mirrors Unix cat's ability to concatenate multiple files.
//
// The response format adapts to the request: a single path returns a plain
// object, while multiple paths return an array. This convention (also used by
// the CLI's JSON output) keeps single-document responses simple while still
// supporting batch operations.
//
// Version and include_deleted parameters apply uniformly to all requested paths,
// which matches typical use cases where you either want current versions of
// several documents or are examining historical state at a point in time.
func (h *handlers) readDocumentTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	paths := getStrings(req, "paths")
	if len(paths) == 0 {
		return mcp.NewToolResultError("paths is required"), nil
	}

	version := getInt(req, "version", 0)
	includeDeleted := getBool(req, "include_deleted", false)
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:read", "read").Author(author)
	if len(paths) == 1 {
		l.Path(paths[0])
	} else {
		l.Detail("paths", paths)
	}
	defer func() { l.Detail("count", len(paths)).Write(nil) }()

	var docs []store.DocJSON
	for _, path := range paths {
		var doc *store.Document
		var err error
		if version > 0 {
			doc, err = h.svc.Version(ctx, path, version)
		} else {
			doc, _, err = h.svc.Resolve(ctx, path, includeDeleted)
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		docs = append(docs, doc.ToJSON(true))
	}

	// Return single object for single path, array for multiple
	if len(docs) == 1 {
		return jsonResult(docs[0])
	}
	return jsonResult(docs)
}

// writeDocument handles llmd_write tool calls.
//
// This is the primary document creation and update tool. Unlike read operations,
// author is strictly required (not defaulted) because every write must be
// attributable for audit purposes. The author typically identifies the LLM agent
// (e.g., "claude-code") so that document history clearly shows which system made
// each change.
//
// The optional message parameter allows semantic commit messages, which helps
// when reviewing document history to understand why changes were made, not just
// what changed.
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
//
// Supports two deletion modes: soft delete of an entire document (the common
// case) or hard delete of a specific version. Soft deletes are recoverable via
// llmd_restore; version deletes are permanent and typically used for cleanup.
//
// The path parameter accepts either document paths or 8-character version keys.
// When given a key, the function resolves it to find the specific version and
// deletes just that version. This distinction is important: "delete by path"
// soft-deletes the document, while "delete by key" hard-deletes that version.
//
// The author parameter is required for audit trail purposes, even though
// deletion is technically destructive - knowing who deleted what is valuable
// for debugging and potential recovery discussions.
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
//
// Restores a soft-deleted document, making it visible again in normal listings
// and readable without the include_deleted flag. This is the counterpart to
// deleteDocument's soft delete behaviour.
//
// The path parameter accepts both document paths and 8-character keys, using
// Resolve with includeDeleted=true since we're specifically trying to find a
// deleted document. The function logs both the input (path or key) and the
// resolved path to maintain clear audit trails.
//
// Author is required to track who restored the document, completing the audit
// trail: we know who deleted it and who brought it back.
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
//
// Renames or relocates a document within the store. This is a metadata-only
// operation that updates the document's path without creating a new version
// or modifying content. All version history is preserved under the new path.
//
// Both from and to are required parameters - there's no sensible default for
// a rename operation. Author is required for the audit trail.
//
// Unlike Unix mv, this doesn't support moving multiple sources to a directory;
// each move is a single source-to-destination operation. This simplicity avoids
// ambiguity about what happens when paths conflict or when the destination
// exists.
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
//
// Returns the version history of a document, which is essential for LLMs that
// need to understand how a document has evolved or want to reference a specific
// prior version. Each version includes metadata (author, message, timestamp)
// but excludes content to keep the response size manageable.
//
// The path parameter accepts both document paths and 8-character keys, resolved
// to the canonical path before fetching history. The limit parameter caps the
// number of versions returned, which is useful for documents with long histories
// where the LLM only needs recent changes.
//
// The include_deleted flag allows viewing history of soft-deleted documents,
// which supports recovery workflows where an LLM needs to understand what a
// document contained before deciding whether to restore it.
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
//
// Provides search-and-replace editing, which is often more efficient than
// full document rewrites for small changes. The LLM specifies the exact text
// to find (old) and what to replace it with (new). If new is empty, the old
// text is deleted.
//
// This approach has several advantages over full writes: it's more efficient
// for bandwidth (only the change is transmitted), it clearly communicates
// intent (what specifically is being changed), and it fails safely if the
// expected text isn't found (avoiding accidental overwrites of concurrent
// changes).
//
// The edit is delegated to internal/edit which handles the matching and
// replacement logic, ensuring consistency with CLI edit behaviour.
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
