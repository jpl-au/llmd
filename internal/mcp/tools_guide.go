// tools_guide.go implements the MCP tool for accessing help content.
//
// The guide tool provides LLMs with documentation about llmd commands
// and usage patterns, enabling self-service help without external lookups.

package mcp

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/guide"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// getGuide handles llmd_guide tool calls.
func (h *handlers) getGuide(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:revive // ctx for future use
	topic := getString(req, "topic", "")

	content, err := guide.Get(topic)

	log.Event("mcp:guide", "read").Author("mcp").Detail("topic", topic).Write(err)

	if err != nil {
		// If topic not found, return list of available topics
		topics, listErr := guide.List()
		if listErr != nil {
			return nil, fmt.Errorf("listing guides: %w", listErr)
		}
		return jsonResult(map[string]any{
			"error":            err.Error(),
			"available_topics": topics,
		})
	}

	return mcp.NewToolResultText(content), nil
}
