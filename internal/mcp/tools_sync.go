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
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	dir := h.svc.FilesDir()
	if _, statErr := os.Stat(dir); errors.Is(statErr, fs.ErrNotExist) {
		return mcp.NewToolResultText("no files directory found"), nil
	}

	var err error
	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	l := log.Event("mcp:sync", "sync").Author(author)
	defer func() { l.Write(err) }()

	docs, err := h.svc.List(ctx, "", false, false)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	db := make(map[string]string, len(docs))
	for _, d := range docs {
		db[d.Path] = d.Content
	}

	opts := sync.Options{
		DryRun: getBool(req, "dry_run", false),
		Author: author,
		Msg:    getString(req, "message", ""),
	}

	var buf bytes.Buffer
	syncResult, err := sync.Run(ctx, &buf, h.svc, dir, db, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("added", syncResult.Added).Detail("updated", syncResult.Updated)

	return jsonResult(map[string]any{
		"updated": syncResult.Updated,
		"added":   syncResult.Added,
		"dry_run": opts.DryRun,
	})
}
