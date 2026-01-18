// tools_diff.go implements the MCP tool for comparing document versions.
//
// Diff enables LLMs to understand what changed between versions or between
// two different documents, supporting review and audit workflows.

package mcp

import (
	"context"

	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// diffDocuments handles llmd_diff tool calls.
func (h *handlers) diffDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}

	opts := diff.Options{
		Path2:          getString(req, "path2", ""),
		Version1:       getInt(req, "version1", 0),
		Version2:       getInt(req, "version2", 0),
		IncludeDeleted: getBool(req, "include_deleted", false),
	}
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:diff", "diff").Author(author).Path(path)
	defer func() { l.Write(err) }()

	r, err := h.svc.Diff(ctx, path, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]string{
		"old":  r.Old,
		"new":  r.New,
		"diff": r.Format(false),
	})
}
