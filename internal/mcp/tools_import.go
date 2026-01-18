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

	opts := importer.Options{
		Prefix: getString(req, "prefix", ""),
		Flat:   getBool(req, "flat", false),
		Hidden: getBool(req, "hidden", false),
		DryRun: getBool(req, "dry_run", false),
		Author: author,
	}

	l := log.Event("mcp:import", "import").Author(author).Detail("source", path)
	defer func() { l.Write(err) }()

	var buf bytes.Buffer
	importResult, err := importer.Run(ctx, &buf, h.svc, path, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", importResult.Imported)

	return jsonResult(map[string]any{
		"imported": importResult.Imported,
		"paths":    importResult.Paths,
		"dry_run":  opts.DryRun,
	})
}
