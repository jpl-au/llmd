// tools_export.go implements the MCP tool for exporting documents.
//
// Export writes documents from the llmd store to the filesystem. It has
// security implications (path traversal, overwriting files) and different
// failure modes (filesystem permissions, disk space).

package mcp

import (
	"bytes"
	"context"

	"github.com/jpl-au/llmd/internal/exporter"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// exportFiles handles llmd_export tool calls.
func (h *handlers) exportFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	dest, err := req.RequireString("dest")
	if err != nil {
		return mcp.NewToolResultError("dest is required"), nil
	}

	opts := exporter.Options{
		Version: getInt(req, "version", 0),
		Force:   getBool(req, "force", false),
	}
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:export", "export").Author(author).Path(path).Detail("dest", dest)
	defer func() { l.Write(err) }()

	var buf bytes.Buffer
	exportResult, err := exporter.Run(ctx, &buf, h.svc, path, dest, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	l.Detail("count", exportResult.Exported)

	return jsonResult(map[string]any{
		"exported": exportResult.Exported,
		"paths":    exportResult.Paths,
	})
}
