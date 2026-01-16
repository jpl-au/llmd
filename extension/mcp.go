// mcp.go defines types for MCP tool registration by extensions.
//
// Separated from extension.go to isolate MCP-specific concerns. Not all
// extensions need MCP tools - some only provide CLI commands.
//
// Design: MCPTool pairs the tool definition with its handler, enabling
// extensions to register complete tool implementations. The handler receives
// both Go context (for cancellation) and extension Context (for service access).

package extension

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTool pairs an MCP tool definition with its handler.
type MCPTool struct {
	Tool    mcp.Tool
	Handler MCPHandler
}

// MCPHandler processes MCP tool requests.
// The Context provides access to the document service and database.
type MCPHandler func(ctx context.Context, extCtx Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
