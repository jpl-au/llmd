// Package mcp implements the Model Context Protocol server, exposing llmd
// operations to LLMs. This enables AI assistants to read, write, and manage
// documents through a standardised protocol.
package mcp

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/repo"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Version is advertised to clients for capability negotiation.
const Version = "1.0.0"

// ErrNotInitialised is returned by tools when the store has not been initialised.
// The LLM should call llmd_init to create a store before using other tools.
const ErrNotInitialised = "store not initialised - call llmd_init first"

// Serve starts the MCP server over stdio, enabling LLM integration.
// Uses stdio transport for compatibility with Claude Desktop and other MCP clients.
//
// Design: The server starts successfully even if no store exists. This allows
// LLMs to call llmd_init to create a store, rather than failing with an opaque
// error. Tools that require a store return ErrNotInitialised with clear guidance.
func Serve(db string) error {
	// Log to stderr; stdout is reserved for MCP JSON-RPC messages
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	h := &handlers{db: db}

	// Try to open existing store; nil service is OK (uninitialised mode)
	svc, err := document.New(db)
	if err != nil && !errors.Is(err, repo.ErrNotInitialised) {
		// Real error (not just uninitialised)
		slog.Error("failed to open store", "error", err)
		return err
	}
	if err == nil {
		h.svc = svc
		defer svc.Close()
	} else {
		slog.Info("llmd not initialised, starting in uninitialised mode - call llmd_init to create store")
	}

	s := server.NewMCPServer(
		"llmd",
		Version,
		server.WithResourceCapabilities(true, false),
		server.WithToolCapabilities(true),
	)

	registerResources(s, h)
	registerTools(s, h)

	slog.Info("llmd MCP server ready", "version", Version, "transport", "stdio")

	err = server.ServeStdio(s)
	if errors.Is(err, context.Canceled) {
		slog.Info("server stopped")
		return nil
	}
	return err
}

// handlers provides MCP request handlers with access to the document store.
// The svc field may be nil if the store has not been initialised.
type handlers struct {
	db  string            // database name for init
	svc *document.Service // nil if not initialised
}

// requireInit returns an error result if the store is not initialised.
// Tools that require a store should call this first.
func (h *handlers) requireInit() *mcp.CallToolResult {
	if h.svc == nil {
		return mcp.NewToolResultError(ErrNotInitialised)
	}
	return nil
}

// registerResources adds URI-based resource access for direct document reading.
func registerResources(s *server.MCPServer, h *handlers) {
	// Document content by path
	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"llmd://documents/{path}",
			"Document",
			mcp.WithTemplateDescription("Read document content by path"),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		h.readDocument,
	)

	// Document content by path and version
	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"llmd://documents/{path}/v/{version}",
			"Document Version",
			mcp.WithTemplateDescription("Read specific version of a document"),
			mcp.WithTemplateMIMEType("text/markdown"),
		),
		h.readDocumentVersion,
	)
}

// registerTools exposes llmd operations as MCP tools for LLM invocation.
func registerTools(s *server.MCPServer, h *handlers) {
	// Init - works without existing store
	s.AddTool(
		mcp.NewTool("llmd_init",
			mcp.WithDescription("Initialise a new llmd document store. Call this first if other tools return 'store not initialised'."),
			mcp.WithBoolean("local", mcp.Description("If true, database is gitignored (not committed to version control)")),
		),
		h.initStore,
	)

	// List documents
	s.AddTool(
		mcp.NewTool("llmd_list",
			mcp.WithDescription("List documents in the store"),
			mcp.WithString("prefix", mcp.Description("Filter by path prefix")),
			mcp.WithBoolean("include_deleted", mcp.Description("Include soft-deleted documents")),
			mcp.WithBoolean("deleted_only", mcp.Description("Show only deleted documents")),
			mcp.WithString("tag", mcp.Description("Filter by tag")),
		),
		h.listDocuments,
	)

	// Read document
	s.AddTool(
		mcp.NewTool("llmd_read",
			mcp.WithDescription("Read a document's content"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("version", mcp.Description("Specific version to read (default: latest)")),
			mcp.WithBoolean("include_deleted", mcp.Description("Allow reading deleted documents")),
		),
		h.readDocumentTool,
	)

	// Write document
	s.AddTool(
		mcp.NewTool("llmd_write",
			mcp.WithDescription("Write content to a document (create or update)"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Document content")),
			mcp.WithString("author", mcp.Required(), mcp.Description("Author attribution")),
			mcp.WithString("message", mcp.Description("Version message")),
		),
		h.writeDocument,
	)

	// Delete document
	s.AddTool(
		mcp.NewTool("llmd_delete",
			mcp.WithDescription("Soft delete a document (recoverable via llmd_restore)"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("version", mcp.Description("Delete only this specific version (default: all versions)")),
		),
		h.deleteDocument,
	)

	// Restore document
	s.AddTool(
		mcp.NewTool("llmd_restore",
			mcp.WithDescription("Restore a soft-deleted document"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
		),
		h.restoreDocument,
	)

	// Move document
	s.AddTool(
		mcp.NewTool("llmd_move",
			mcp.WithDescription("Move/rename a document"),
			mcp.WithString("from", mcp.Required(), mcp.Description("Source path")),
			mcp.WithString("to", mcp.Required(), mcp.Description("Destination path")),
		),
		h.moveDocument,
	)

	// Search documents
	s.AddTool(
		mcp.NewTool("llmd_search",
			mcp.WithDescription("Full-text search across documents"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
			mcp.WithString("prefix", mcp.Description("Limit search to path prefix")),
			mcp.WithBoolean("include_deleted", mcp.Description("Include deleted documents")),
			mcp.WithBoolean("deleted_only", mcp.Description("Search only deleted documents")),
		),
		h.searchDocuments,
	)

	// History
	s.AddTool(
		mcp.NewTool("llmd_history",
			mcp.WithDescription("Get version history for a document"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithNumber("limit", mcp.Description("Maximum versions to return")),
			mcp.WithBoolean("include_deleted", mcp.Description("Include deleted versions")),
		),
		h.historyDocument,
	)

	// Diff
	s.AddTool(
		mcp.NewTool("llmd_diff",
			mcp.WithDescription("Show differences between document versions or two documents"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("path2", mcp.Description("Second document path (for comparing two documents)")),
			mcp.WithNumber("version1", mcp.Description("First version to compare")),
			mcp.WithNumber("version2", mcp.Description("Second version to compare")),
			mcp.WithBoolean("include_deleted", mcp.Description("Allow diffing deleted documents")),
		),
		h.diffDocuments,
	)

	// Edit
	s.AddTool(
		mcp.NewTool("llmd_edit",
			mcp.WithDescription("Edit a document via search/replace (replaces first occurrence)"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("old", mcp.Required(), mcp.Description("Text to find")),
			mcp.WithString("new", mcp.Description("Text to replace with")),
			mcp.WithString("author", mcp.Required(), mcp.Description("Author attribution")),
			mcp.WithString("message", mcp.Description("Version message")),
		),
		h.editDocument,
	)

	// Glob
	s.AddTool(
		mcp.NewTool("llmd_glob",
			mcp.WithDescription("List document paths matching a glob pattern"),
			mcp.WithString("pattern", mcp.Description("Glob pattern (supports *, **, ?)")),
		),
		h.globDocuments,
	)

	// Config Get
	s.AddTool(
		mcp.NewTool("llmd_config_get",
			mcp.WithDescription("Get a configuration value"),
			mcp.WithString("key", mcp.Description("Config key (author.name, author.email, sync.files) or empty for all")),
		),
		h.configGet,
	)

	// Config Set
	s.AddTool(
		mcp.NewTool("llmd_config_set",
			mcp.WithDescription("Set a configuration value"),
			mcp.WithString("key", mcp.Required(), mcp.Description("Config key (author.name, author.email, sync.files)")),
			mcp.WithString("value", mcp.Required(), mcp.Description("Value to set")),
		),
		h.configSet,
	)

	// Import
	s.AddTool(
		mcp.NewTool("llmd_import",
			mcp.WithDescription("Import markdown files from filesystem into the store"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Filesystem path to import from")),
			mcp.WithString("prefix", mcp.Description("Target path prefix in store")),
			mcp.WithString("author", mcp.Required(), mcp.Description("Author attribution")),
			mcp.WithBoolean("flat", mcp.Description("Flatten directory structure")),
			mcp.WithBoolean("hidden", mcp.Description("Include hidden files/directories")),
			mcp.WithBoolean("dry_run", mcp.Description("Show what would be imported without importing")),
		),
		h.importFiles,
	)

	// Export
	s.AddTool(
		mcp.NewTool("llmd_export",
			mcp.WithDescription("Export documents from store to filesystem"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path or prefix (use trailing / for prefix)")),
			mcp.WithString("dest", mcp.Required(), mcp.Description("Filesystem destination path")),
			mcp.WithNumber("version", mcp.Description("Export specific version (for single doc)")),
			mcp.WithBoolean("force", mcp.Description("Overwrite existing files")),
		),
		h.exportFiles,
	)

	// Sync
	s.AddTool(
		mcp.NewTool("llmd_sync",
			mcp.WithDescription("Sync filesystem changes back to database"),
			mcp.WithString("author", mcp.Required(), mcp.Description("Author attribution")),
			mcp.WithBoolean("dry_run", mcp.Description("Show what would be synced without syncing")),
			mcp.WithString("message", mcp.Description("Commit message for synced documents")),
		),
		h.syncFiles,
	)

	// Guide
	s.AddTool(
		mcp.NewTool("llmd_guide",
			mcp.WithDescription("Get help/guide content for llmd commands"),
			mcp.WithString("topic", mcp.Description("Guide topic (e.g., 'write', 'ls', 'find') or empty for index")),
		),
		h.getGuide,
	)
	// Tag Add
	s.AddTool(
		mcp.NewTool("llmd_tag_add",
			mcp.WithDescription("Add a tag to a document"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("tag", mcp.Required(), mcp.Description("Tag to add")),
		),
		h.tagAdd,
	)

	// Tag Remove
	s.AddTool(
		mcp.NewTool("llmd_tag_remove",
			mcp.WithDescription("Remove a tag from a document"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("tag", mcp.Required(), mcp.Description("Tag to remove")),
		),
		h.tagRemove,
	)

	// List Tags
	s.AddTool(
		mcp.NewTool("llmd_tags",
			mcp.WithDescription("List tags for a document or all tags"),
			mcp.WithString("path", mcp.Description("Document path (optional, list all if empty)")),
		),
		h.listTags,
	)

	// Sed
	s.AddTool(
		mcp.NewTool("llmd_sed",
			mcp.WithDescription("Edit a document using sed-style substitution (e.g., s/old/new/)"),
			mcp.WithString("path", mcp.Required(), mcp.Description("Document path")),
			mcp.WithString("expression", mcp.Required(), mcp.Description("Sed expression (e.g., s/old/new/ or s/old/new/g for global)")),
			mcp.WithString("author", mcp.Required(), mcp.Description("Author attribution")),
			mcp.WithString("message", mcp.Description("Version message")),
		),
		h.sedDocument,
	)

	// Grep
	s.AddTool(
		mcp.NewTool("llmd_grep",
			mcp.WithDescription("Search documents using regex. For FTS5 full-text search, use llmd_search"),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Regex pattern (e.g., 'error|warn', 'TODO.*fix', '[0-9]{3}')")),
			mcp.WithString("path", mcp.Description("Limit search to path prefix")),
			mcp.WithBoolean("ignore_case", mcp.Description("Case insensitive search")),
			mcp.WithBoolean("paths_only", mcp.Description("Only return matching paths")),
			mcp.WithBoolean("include_deleted", mcp.Description("Include deleted documents")),
			mcp.WithBoolean("deleted_only", mcp.Description("Search only deleted documents")),
		),
		h.grepDocuments,
	)

	// Link
	s.AddTool(
		mcp.NewTool("llmd_link",
			mcp.WithDescription("Create or list document links"),
			mcp.WithString("from", mcp.Description("Source document path (required for creating)")),
			mcp.WithString("to", mcp.Description("Target document path (required for creating)")),
			mcp.WithString("tag", mcp.Description("Link tag for categorisation")),
			mcp.WithBoolean("list", mcp.Description("List links for 'from' path")),
			mcp.WithBoolean("orphan", mcp.Description("List documents with no links")),
		),
		h.linkDocuments,
	)

	// Unlink
	s.AddTool(
		mcp.NewTool("llmd_unlink",
			mcp.WithDescription("Remove a link by ID or all links with a tag"),
			mcp.WithString("id", mcp.Description("Link ID to remove")),
			mcp.WithString("tag", mcp.Description("Remove all links with this tag")),
		),
		h.unlinkDocuments,
	)
}

// readDocument handles llmd://documents/{path} resource requests.
func (h *handlers) readDocument(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return h.readDocumentResource(ctx, req.Params.URI)
}

// readDocumentVersion handles llmd://documents/{path}/v/{version} resource requests.
func (h *handlers) readDocumentVersion(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return h.readDocumentResource(ctx, req.Params.URI)
}
