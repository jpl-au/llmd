// tools_sed.go implements the MCP tool for sed-style document editing.
//
// Sed provides a familiar interface for text transformations, complementing
// the search/replace edit tool with regex-based substitutions.

package mcp

import (
	"bytes"
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/sed"
	"github.com/mark3labs/mcp-go/mcp"
)

// sedDocument handles llmd_sed tool calls.
func (h *handlers) sedDocument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	expr, err := req.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError("expression is required"), nil
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	opts := sed.Options{
		Author:  author,
		Message: getString(req, "message", ""),
	}

	l := log.Event("mcp:sed", "edit").Author(opts.Author).Path(path)
	defer func() { l.Write(err) }()

	var buf bytes.Buffer
	_, err = sed.Run(ctx, &buf, h.svc, path, expr, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("edited %s", path)), nil
}
