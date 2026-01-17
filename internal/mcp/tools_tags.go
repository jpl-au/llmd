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
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}
	tag, err := req.RequireString("tag")
	if err != nil {
		return mcp.NewToolResultError("tag is required"), nil //nolint:nilerr
	}

	err = h.svc.Tag(ctx, path, tag, store.NewTagOptions())

	log.Event("mcp:tag_add", "tag").Author("mcp").Path(path).Detail("tag", tag).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("added tag %q to %s", tag, path)), nil
}

// tagRemove handles llmd_tag_remove tool calls.
func (h *handlers) tagRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path is required"), nil //nolint:nilerr
	}
	tag, err := req.RequireString("tag")
	if err != nil {
		return mcp.NewToolResultError("tag is required"), nil //nolint:nilerr
	}

	err = h.svc.Untag(ctx, path, tag, store.NewTagOptions())

	log.Event("mcp:tag_remove", "untag").Author("mcp").Path(path).Detail("tag", tag).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("removed tag %q from %s", tag, path)), nil
}

// listTags handles llmd_tags tool calls.
func (h *handlers) listTags(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	path := getString(req, "path", "")

	tags, err := h.svc.ListTags(ctx, path, store.NewTagOptions())

	log.Event("mcp:tags", "list_tags").Author("mcp").Path(path).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(tags)
}
