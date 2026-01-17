// tools_import.go implements the MCP tool for importing files.
//
// Import brings external markdown files into the llmd store. It interacts
// with the filesystem and has security implications (path traversal).
//
// Design: Supports dry-run mode for LLMs to preview changes before committing.

package mcp

import (
	"bytes"
	"context"

	"github.com/jpl-au/llmd/internal/importer"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// importFiles handles llmd_import tool calls.
func (h *handlers) importFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil //nolint:nilerr
	}

	opts := importer.Options{
		Prefix: getString(req, "prefix", ""),
		Flat:   getBool(req, "flat", false),
		Hidden: getBool(req, "hidden", false),
		DryRun: getBool(req, "dry_run", false),
		Author: author,
	}

	var buf bytes.Buffer
	result, err := importer.Run(ctx, &buf, h.svc, path, opts)

	log.Event("mcp:import", "import").Author(author).Detail("source", path).Detail("count", result.Imported).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"imported": result.Imported,
		"paths":    result.Paths,
		"dry_run":  opts.DryRun,
	})
}
