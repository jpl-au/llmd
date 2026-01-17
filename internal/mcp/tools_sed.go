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
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}

	expr, err := req.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError("expression is required"), nil //nolint:nilerr
	}

	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil //nolint:nilerr
	}

	opts := sed.Options{
		Author:  author,
		Message: getString(req, "message", ""),
	}

	var buf bytes.Buffer
	_, err = sed.Run(ctx, &buf, h.svc, path, expr, opts)

	log.Event("mcp:sed", "edit").Author(opts.Author).Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("edited %s", path)), nil
}
