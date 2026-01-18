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
	"path"
	"strings"

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
// Supports soft deletion of one or more documents, with all deletions being
// recoverable via llmd_restore. When a single path is provided and it resolves
// to an 8-character key, that specific version is hard-deleted instead.
//
// The version parameter restricts deletion to a specific version number and
// only works with a single path (returns an error if used with multiple paths).
// This prevents accidental mass deletion of specific versions across documents.
//
// The response format adapts to the request: single path returns a plain text
// confirmation, while multiple paths return a JSON array of deletion results.
func (h *handlers) deleteDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	paths := getStrings(req, "paths")
	if len(paths) == 0 {
		return mcp.NewToolResultError("paths is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	version := getInt(req, "version", 0)

	// Version flag only works with single path
	if len(paths) > 1 && version > 0 {
		return mcp.NewToolResultError("version parameter cannot be used with multiple paths"), nil
	}

	l := log.Event("mcp:delete", "delete").Author(author)
	if len(paths) == 1 {
		l.Path(paths[0])
	} else {
		l.Detail("paths", paths)
	}
	defer func() { l.Detail("count", len(paths)).Write(nil) }()

	// Single path mode: preserve existing key resolution and version logic
	if len(paths) == 1 {
		inputPath := paths[0]

		// For simple delete (no version), resolve as path or key
		if version == 0 {
			doc, isKey, err := h.svc.Resolve(ctx, inputPath, false)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if isKey {
				// Resolved as key - delete that specific version
				l.Resolved(doc.Path).Version(doc.Version).Detail("key", inputPath)
				if err := h.svc.DeleteVersion(ctx, doc.Path, doc.Version); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf("deleted %s (version %d, key %s)", doc.Path, doc.Version, inputPath)), nil
			}
			// Resolved as path
			if inputPath != doc.Path {
				l.Resolved(doc.Path)
			}
			if err := h.svc.Delete(ctx, doc.Path); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("deleted %s", doc.Path)), nil
		}

		// Version-specific deletion
		l.Version(version)
		if err := h.svc.DeleteVersion(ctx, inputPath, version); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("deleted %s (version %d)", inputPath, version)), nil
	}

	// Multiple paths mode
	type deleteResult struct {
		Path    string `json:"path"`
		Deleted bool   `json:"deleted"`
	}
	var results []deleteResult

	for _, p := range paths {
		if err := h.svc.Delete(ctx, p); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("delete %s: %v", p, err)), nil
		}
		results = append(results, deleteResult{Path: p, Deleted: true})
	}

	return jsonResult(results)
}

// restoreDocument handles llmd_restore tool calls.
//
// Restores one or more soft-deleted documents, making them visible again in
// normal listings. This is the counterpart to deleteDocument's soft delete.
//
// Each path is resolved (supporting both document paths and 8-character keys)
// with includeDeleted=true since we're specifically trying to find deleted
// documents. The response format adapts: single path returns text confirmation,
// multiple paths return a JSON array of results.
func (h *handlers) restoreDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	paths := getStrings(req, "paths")
	if len(paths) == 0 {
		return mcp.NewToolResultError("paths is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	l := log.Event("mcp:restore", "restore").Author(author)
	if len(paths) == 1 {
		l.Path(paths[0])
	} else {
		l.Detail("paths", paths)
	}
	defer func() { l.Detail("count", len(paths)).Write(nil) }()

	// Single path mode
	if len(paths) == 1 {
		inputPath := paths[0]
		doc, isKey, err := h.svc.Resolve(ctx, inputPath, true)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("%q: %v", inputPath, err)), nil
		}

		if isKey {
			l.Detail("key", inputPath)
		}
		if inputPath != doc.Path {
			l.Resolved(doc.Path)
		}

		if err := h.svc.Restore(ctx, doc.Path); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if isKey {
			return mcp.NewToolResultText(fmt.Sprintf("restored %s (from key %s)", doc.Path, inputPath)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("restored %s", doc.Path)), nil
	}

	// Multiple paths mode
	type restoreResult struct {
		Path string `json:"path"`
		Key  string `json:"key,omitempty"`
	}
	var results []restoreResult

	for _, inputPath := range paths {
		doc, isKey, err := h.svc.Resolve(ctx, inputPath, true)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("%q: %v", inputPath, err)), nil
		}

		if err := h.svc.Restore(ctx, doc.Path); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("restore %s: %v", doc.Path, err)), nil
		}

		result := restoreResult{Path: doc.Path}
		if isKey {
			result.Key = inputPath
		}
		results = append(results, result)
	}

	return jsonResult(results)
}

// revertDocument handles llmd_revert tool calls.
//
// Reverts a document to a previous version by creating a new version with the
// old content. This is a forward-moving operation that preserves complete
// history - you can see when a revert happened and even revert a revert.
//
// The target version can be specified by:
//   - path + version number: revert docs/api to version 3
//   - key: revert to the specific version identified by the 8-char key
//
// Author is required as this creates a new version in the document's history.
func (h *handlers) revertDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	path := getString(req, "path", "")
	version := getInt(req, "version", 0)
	key := getString(req, "key", "")
	message := getString(req, "message", "")

	// Validate: need either key or (path + version)
	if key == "" && path == "" {
		return mcp.NewToolResultError("either 'key' or 'path' is required"), nil
	}
	if key == "" && version == 0 {
		return mcp.NewToolResultError("either 'key' or 'version' is required"), nil
	}

	l := log.Event("mcp:revert", "revert").Author(author).Path(path).Version(version).Detail("key", key)
	defer func() { l.Write(nil) }()

	var doc *store.Document

	if key != "" {
		// Revert by key
		doc, err = h.svc.ByKey(ctx, key)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("key %q: %v", key, err)), nil
		}
	} else {
		// Revert by path + version
		doc, err = h.svc.Version(ctx, path, version)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("version %d of %q: %v", version, path, err)), nil
		}
	}

	// Build message if not provided
	if message == "" {
		if key != "" {
			message = fmt.Sprintf("Revert to %s", key)
		} else {
			message = fmt.Sprintf("Revert to v%d", version)
		}
	}

	// Write old content as new version
	if err := h.svc.Write(ctx, doc.Path, doc.Content, author, message); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write reverted content: %v", err)), nil
	}

	// Get new version number
	newDoc, err := h.svc.Latest(ctx, doc.Path, false)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get new version: %v", err)), nil
	}

	l.Resolved(doc.Path).ResultVersion(newDoc.Version)

	type revertResult struct {
		Path       string `json:"path"`
		RevertedTo int    `json:"reverted_to"`
		NewVersion int    `json:"new_version"`
		Key        string `json:"key,omitempty"`
		Message    string `json:"message"`
	}

	return jsonResult(revertResult{
		Path:       doc.Path,
		RevertedTo: doc.Version,
		NewVersion: newDoc.Version,
		Key:        doc.Key,
		Message:    message,
	})
}

// moveDocument handles llmd_move tool calls.
//
// Renames or relocates documents within the store. This is a metadata-only
// operation that updates document paths without creating new versions or
// modifying content. All version history is preserved under the new paths.
//
// Supports Unix mv semantics: with multiple sources or a destination ending
// in /, the destination is treated as a prefix and sources are moved under it
// preserving their base names (docs/readme -> archive/readme). With a single
// source and no trailing slash, it's a simple rename.
//
// The response format adapts to the request: single source returns a plain
// object with from/to, while multiple sources return an array of results.
func (h *handlers) moveDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	sources := getStrings(req, "sources")
	if len(sources) == 0 {
		return mcp.NewToolResultError("sources is required"), nil
	}

	dest, err := req.RequireString("dest")
	if err != nil {
		return mcp.NewToolResultError("dest is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	// Determine if this is "move into prefix" mode:
	// - Multiple sources always require prefix mode
	// - Trailing slash signals prefix mode even with single source
	prefixMode := len(sources) > 1 || strings.HasSuffix(dest, "/")
	destPrefix := strings.TrimSuffix(dest, "/")

	l := log.Event("mcp:move", "move").Author(author)
	if len(sources) == 1 {
		l.Path(sources[0])
	} else {
		l.Detail("sources", sources)
	}
	l.Detail("dest", dest)
	defer func() { l.Detail("count", len(sources)).Write(nil) }()

	type moveResult struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	var results []moveResult

	for _, src := range sources {
		var target string
		if prefixMode {
			// Move into prefix: docs/readme -> archive/readme
			base := path.Base(src)
			target = path.Join(destPrefix, base)
		} else {
			// Direct rename
			target = dest
		}

		if err := h.svc.Move(ctx, src, target); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("move %s: %v", src, err)), nil
		}
		results = append(results, moveResult{From: src, To: target})
	}

	// Return single object for single move, array for multiple
	if len(results) == 1 {
		return jsonResult(results[0])
	}
	return jsonResult(results)
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
