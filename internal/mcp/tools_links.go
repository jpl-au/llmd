// tools_links.go implements MCP tools for document relationship management.
//
// Separated from tools_tags.go because links represent relationships between
// documents (graph edges) with different query patterns - traversal, orphan
// detection, and bidirectional lookup.
//
// Design: The link tool combines create, list, and delete operations based
// on parameters. This reduces the tool count for LLMs while maintaining
// full functionality through parameter combinations.

package mcp

import (
	"context"

	"github.com/jpl-au/llmd/internal/log"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// linkDocuments handles llmd_link tool calls.
func (h *handlers) linkDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	from := getString(req, "from", "")
	to := getString(req, "to", "")
	tag := getString(req, "tag", "")
	list := getBool(req, "list", false)
	orphan := getBool(req, "orphan", false)

	// List orphans
	if orphan {
		paths, err := h.svc.ListOrphanLinkPaths(ctx, store.NewLinkOptions())
		log.Event("mcp:link", "list").Author("mcp").Detail("orphan", true).Detail("count", len(paths)).Write(err)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(paths)
	}

	// List links
	if list {
		if from == "" && tag == "" {
			return mcp.NewToolResultError("from path or tag required for listing"), nil
		}

		// List by tag only
		if from == "" {
			links, err := h.svc.ListLinksByTag(ctx, tag, store.NewLinkOptions())
			log.Event("mcp:link", "list").Author("mcp").Detail("tag", tag).Detail("count", len(links)).Write(err)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			js := make([]store.LinkJSON, len(links))
			for i, l := range links {
				js[i] = l.ToJSON()
			}
			return jsonResult(js)
		}

		// List for path
		links, err := h.svc.ListLinks(ctx, from, tag, store.NewLinkOptions())
		log.Event("mcp:link", "list").Author("mcp").Path(from).Detail("tag", tag).Detail("count", len(links)).Write(err)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		js := make([]store.LinkJSON, len(links))
		for i, l := range links {
			js[i] = l.ToJSON()
		}
		return jsonResult(js)
	}

	// Create link
	if from == "" || to == "" {
		return mcp.NewToolResultError("from and to are required for creating links"), nil
	}

	id, err := h.svc.Link(ctx, from, to, tag, store.NewLinkOptions())
	log.Event("mcp:link", "link").Author("mcp").Path(from).Detail("to", to).Detail("tag", tag).Detail("id", id).Write(err)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"id":   id,
		"from": from,
		"to":   to,
		"tag":  tag,
	})
}

// unlinkDocuments handles llmd_unlink tool calls.
func (h *handlers) unlinkDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	id := getString(req, "id", "")
	tag := getString(req, "tag", "")

	// Remove by tag
	if tag != "" {
		n, err := h.svc.UnlinkByTag(ctx, tag, store.NewLinkOptions())
		log.Event("mcp:unlink", "unlink").Author("mcp").Detail("tag", tag).Detail("count", n).Write(err)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(map[string]any{
			"tag":   tag,
			"count": n,
		})
	}

	// Remove by ID
	if id == "" {
		return mcp.NewToolResultError("id or tag is required"), nil
	}

	err := h.svc.UnlinkByID(ctx, id)
	log.Event("mcp:unlink", "unlink").Author("mcp").Detail("id", id).Write(err)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"id":      id,
		"removed": true,
	})
}
