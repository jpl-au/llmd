// tools_import_export.go implements MCP tools for filesystem integration.
//
// Separated because import/export operations interact with the external
// filesystem, unlike other tools that work purely with the database. These
// operations have security implications (path traversal) and different
// failure modes (filesystem permissions, disk space).
//
// Design: Import/export tools support dry-run mode for LLMs to preview
// changes before committing. This enables safer automation workflows.

package mcp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/jpl-au/llmd/guide"
	"github.com/jpl-au/llmd/internal/exporter"
	"github.com/jpl-au/llmd/internal/importer"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sync"
	"github.com/mark3labs/mcp-go/mcp"
)

// importFiles handles llmd_import tool calls.
func (h *handlers) importFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	opts := importer.Options{
		Prefix: getString(req, "prefix", ""),
		Flat:   getBool(req, "flat", false),
		Hidden: getBool(req, "hidden", false),
		DryRun: getBool(req, "dry_run", false),
		Author: "mcp",
	}

	var buf bytes.Buffer
	result, err := importer.Run(ctx, &buf, h.svc, path, opts)

	log.Event("mcp:import", "import").Author("mcp").Detail("source", path).Detail("count", result.Imported).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"imported": result.Imported,
		"paths":    result.Paths,
		"dry_run":  opts.DryRun,
	})
}

// exportFiles handles llmd_export tool calls.
func (h *handlers) exportFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	dest, err := req.RequireString("dest")
	if err != nil {
		return mcp.NewToolResultError("dest is required"), nil //nolint:nilerr
	}

	opts := exporter.Options{
		Version: getInt(req, "version", 0),
		Force:   getBool(req, "force", false),
	}

	var buf bytes.Buffer
	result, err := exporter.Run(ctx, &buf, h.svc, path, dest, opts)

	log.Event("mcp:export", "export").Author("mcp").Path(path).Detail("dest", dest).Detail("count", result.Exported).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"exported": result.Exported,
		"paths":    result.Paths,
	})
}

// syncFiles handles llmd_sync tool calls.
func (h *handlers) syncFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := h.svc.FilesDir()
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return mcp.NewToolResultText("no files directory found"), nil
	}

	docs, err := h.svc.List(ctx, "", false, false)
	if err != nil {
		log.Event("mcp:sync", "sync").Author("mcp").Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	db := make(map[string]string, len(docs))
	for _, d := range docs {
		db[d.Path] = d.Content
	}

	opts := sync.Options{
		DryRun: getBool(req, "dry_run", false),
		Author: "mcp",
		Msg:    getString(req, "message", ""),
	}

	var buf bytes.Buffer
	result, err := sync.Run(ctx, &buf, h.svc, dir, db, opts)

	log.Event("mcp:sync", "sync").Author("mcp").Detail("added", result.Added).Detail("updated", result.Updated).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"updated": result.Updated,
		"added":   result.Added,
		"dry_run": opts.DryRun,
	})
}

// getGuide handles llmd_guide tool calls.
func (h *handlers) getGuide(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:revive // ctx for future use
	topic := getString(req, "topic", "")

	content, err := guide.Get(topic)

	log.Event("mcp:guide", "read").Author("mcp").Detail("topic", topic).Write(err)

	if err != nil {
		// If topic not found, return list of available topics
		topics := guide.List()
		return jsonResult(map[string]any{
			"error":            err.Error(),
			"available_topics": topics,
		})
	}

	return mcp.NewToolResultText(content), nil
}
