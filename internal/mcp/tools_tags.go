// tools_tags.go implements MCP tools for document tagging operations.
//
// Separated from tools_documents.go because tags have independent lifecycle
// and their own query patterns (list all tags, find documents by tag).
//
// Design: Tag operations are idempotent - adding an existing tag or removing
// a non-existent tag succeeds silently. This simplifies LLM workflows that
// may not track current tag state.

package mcp

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// tagAdd handles llmd_tag_add tool calls.
func (h *handlers) tagAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}
	tag, err := req.RequireString("tag")
	if err != nil {
		return mcp.NewToolResultError("tag is required"), nil
	}
	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	l := log.Event("mcp:tag_add", "tag").Author(author).Path(path).Detail("tag", tag)
	defer func() { l.Write(err) }()

	err = h.svc.Tag(ctx, path, tag, store.NewTagOptions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("added tag %q to %s", tag, path)), nil
}

// tagRemove handles llmd_tag_remove tool calls.
func (h *handlers) tagRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil
	}
	tag, err := req.RequireString("tag")
	if err != nil {
		return mcp.NewToolResultError("tag is required"), nil
	}
	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	l := log.Event("mcp:tag_remove", "untag").Author(author).Path(path).Detail("tag", tag)
	defer func() { l.Write(err) }()

	err = h.svc.Untag(ctx, path, tag, store.NewTagOptions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("removed tag %q from %s", tag, path)), nil
}

// listTags handles llmd_tags tool calls.
func (h *handlers) listTags(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	path := getString(req, "path", "")
	author := getString(req, "author", "mcp")

	l := log.Event("mcp:tags", "list_tags").Author(author).Path(path)
	defer func() { l.Write(err) }()

	tags, err := h.svc.ListTags(ctx, path, store.NewTagOptions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(tags)
}
