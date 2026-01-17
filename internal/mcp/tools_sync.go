// tools_sync.go implements the MCP tool for syncing filesystem changes.
//
// Sync detects changes made to exported files on the filesystem and writes
// them back to the database. This enables editing documents with external
// tools while keeping the store as the source of truth.

package mcp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sync"
	"github.com/mark3labs/mcp-go/mcp"
)

// syncFiles handles llmd_sync tool calls.
func (h *handlers) syncFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

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

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil //nolint:nilerr
	}

	opts := sync.Options{
		DryRun: getBool(req, "dry_run", false),
		Author: author,
		Msg:    getString(req, "message", ""),
	}

	var buf bytes.Buffer
	result, err := sync.Run(ctx, &buf, h.svc, dir, db, opts)

	log.Event("mcp:sync", "sync").Author(author).Detail("added", result.Added).Detail("updated", result.Updated).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"updated": result.Updated,
		"added":   result.Added,
		"dry_run": opts.DryRun,
	})
}
