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
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	from := getString(req, "from", "")
	to := getString(req, "to", "")
	tag := getString(req, "tag", "")
	list := getBool(req, "list", false)
	orphan := getBool(req, "orphan", false)
	author := getString(req, "author", "mcp") // Optional for list operations, required for create

	// List orphans
	if orphan {
		l := log.Event("mcp:link", "list").Author(author).Detail("orphan", true)
		defer func() { l.Write(err) }()

		var paths []string
		paths, err = h.svc.ListOrphanLinkPaths(ctx, store.NewLinkOptions())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		l.Detail("count", len(paths))
		return jsonResult(paths)
	}

	// List links
	if list {
		if from == "" && tag == "" {
			return mcp.NewToolResultError("from path or tag required for listing"), nil
		}

		// List by tag only
		if from == "" {
			l := log.Event("mcp:link", "list").Author(author).Detail("tag", tag)
			defer func() { l.Write(err) }()

			var links []store.Link
			links, err = h.svc.ListLinksByTag(ctx, tag, store.NewLinkOptions())
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			l.Detail("count", len(links))
			js := make([]store.LinkJSON, len(links))
			for i, lnk := range links {
				js[i] = lnk.ToJSON()
			}
			return jsonResult(js)
		}

		// List for path
		l := log.Event("mcp:link", "list").Author(author).Path(from).Detail("tag", tag)
		defer func() { l.Write(err) }()

		var links []store.Link
		links, err = h.svc.ListLinks(ctx, from, tag, store.NewLinkOptions())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		l.Detail("count", len(links))
		js := make([]store.LinkJSON, len(links))
		for i, lnk := range links {
			js[i] = lnk.ToJSON()
		}
		return jsonResult(js)
	}

	// Create link - author is required (must not be default)
	if from == "" || to == "" {
		return mcp.NewToolResultError("from and to are required for creating links"), nil
	}

	if author == "mcp" {
		// Check if author was explicitly provided or just defaulted
		if _, err := req.RequireString("author"); err != nil {
			return mcp.NewToolResultError("author is required for creating links"), nil
		}
	}

	l := log.Event("mcp:link", "link").Author(author).Path(from).Detail("to", to).Detail("tag", tag)
	defer func() { l.Write(err) }()

	id, err := h.svc.Link(ctx, from, to, tag, store.NewLinkOptions())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	l.Detail("id", id)

	return jsonResult(map[string]any{
		"id":   id,
		"from": from,
		"to":   to,
		"tag":  tag,
	})
}

// unlinkDocuments handles llmd_unlink tool calls.
func (h *handlers) unlinkDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	id := getString(req, "id", "")
	tag := getString(req, "tag", "")

	// Remove by tag
	if tag != "" {
		l := log.Event("mcp:unlink", "unlink").Author(author).Detail("tag", tag)
		defer func() { l.Write(err) }()

		var n int64
		n, err = h.svc.UnlinkByTag(ctx, tag, store.NewLinkOptions())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		l.Detail("count", n)
		return jsonResult(map[string]any{
			"tag":   tag,
			"count": n,
		})
	}

	// Remove by ID
	if id == "" {
		return mcp.NewToolResultError("id or tag is required"), nil
	}

	l := log.Event("mcp:unlink", "unlink").Author(author).Detail("id", id)
	defer func() { l.Write(err) }()

	err = h.svc.UnlinkByID(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"id":      id,
		"removed": true,
	})
}
