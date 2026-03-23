// mcp.go exposes llmd commands as MCP (Model Context Protocol) tools over
// a stdio transport. AI agents like Claude connect to "llmd mcp" as an
// MCP server and call tools like "cat", "write", "grep", etc.
//
// The flow:
//  1. Walk sdk.AllCommands(), skip any not marked MCP, register the rest
//     as MCP tools with a single generic input schema (args + content).
//  2. Block on server.Run reading JSON-RPC from stdin, writing to stdout.
//
// Every tool uses the same input shape (toolInput) because plugins already
// parse their own flags from args - there's no benefit to per-tool schema.
// The Content field is piped to the command as stdin, which is how "write"
// and "edit" receive document bodies.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/app"
	"github.com/jpl-au/llmd/internal/telemetry"
	"github.com/jpl-au/llmd/sdk"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var mcpSpec = sdk.Command{
	Name: "mcp", Desc: "Start MCP stdio server for AI agent integration", Usage: "mcp",
	Streams: true,
}

// toolInput is the MCP input schema shared by all tools.
// Args are passed directly to the plugin's Exec as command-line arguments.
// Content, when non-empty, is delivered as stdin (used by write, edit).
// Author identifies who is making the change - the LLM or agent should
// supply this so mutations are attributed correctly.
type toolInput struct {
	Args    []string `json:"args"    jsonschema:"command arguments"`
	Content string   `json:"content" jsonschema:"document content (for write/edit)"`
	Author  string   `json:"author"  jsonschema:"author name for attributing changes"`
}

// mcpCmd starts an MCP server on stdin/stdout and blocks until the
// connection closes (client disconnects or stdin reaches EOF).
func mcpCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	author := ctx.Author

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "llmd",
		Version: app.Tag,
	}, nil)

	for _, cmd := range sdk.AllCommands() {
		if !cmd.MCP {
			continue
		}
		registerTool(server, cmd, author)
	}

	telemetry.Emit(telemetry.Entry{
		Source:  "mcp",
		Event:   "start",
		Command: "mcp",
		Success: true,
	})

	err := server.Run(context.Background(), &mcp.StdioTransport{})

	telemetry.Emit(telemetry.Entry{
		Source:  "mcp",
		Event:   "stop",
		Command: "mcp",
		Success: err == nil,
		Error:   telemetry.ErrStr(err),
	})

	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}
	return nil, nil
}

// registerTool adds a single llmd command as an MCP tool. The tool name
// is cmd.MCPName if set (to avoid collisions like "grep" → "llmd_grep"),
// otherwise cmd.Name. The description includes usage and flag docs so
// the LLM knows how to call it.
func registerTool(server *mcp.Server, cmd *sdk.Command, author string) {
	name := cmd.Name
	if cmd.MCPName != "" {
		name = cmd.MCPName
	}

	var desc strings.Builder
	desc.WriteString(cmd.Desc)
	if cmd.Usage != "" {
		desc.WriteString("\n\nUsage: llmd " + cmd.Usage)
	}
	if len(cmd.Flags) > 0 {
		desc.WriteString("\n\nFlags:")
		for _, f := range cmd.Flags {
			if f.Short != "" {
				desc.WriteString(fmt.Sprintf("\n  -%s, --%s  %s", f.Short, f.Name, f.Desc))
			} else {
				desc.WriteString(fmt.Sprintf("\n  --%s  %s", f.Name, f.Desc))
			}
		}
	}

	cmdName := cmd.Name

	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: desc.String(),
	}, func(ctx context.Context, req *mcp.CallToolRequest, input toolInput) (*mcp.CallToolResult, any, error) {
		var stdin []byte
		if input.Content != "" {
			stdin = []byte(input.Content)
		}

		// Use the author from the tool call, or fall back to the
		// server-level author (from --author on the mcp command).
		// MCP callers are non-interactive - no config author fallback.
		a := input.Author
		if a == "" {
			a = author
		}

		result, err := sdk.Dispatch(ctx, cmdName, input.Args, a, stdin, "")
		telemetry.Emit(telemetry.Entry{
			Source:  "mcp",
			Command: cmdName,
			Args:    input.Args,
			Author:  a,
			Success: err == nil,
			Error:   telemetry.ErrStr(err),
		})
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: responseText(result)}},
		}, nil, nil
	})
}

// responseText converts an sdk.Response to plain text for MCP TextContent.
// For Result types with both Text and Data, we prefer Text (human-readable).
func responseText(r sdk.Response) string {
	switch v := r.(type) {
	case sdk.Text:
		return string(v)
	case sdk.Result:
		if v.Text != "" {
			return v.Text
		}
		b, _ := json.Marshal(v.Data)
		return string(b)
	case sdk.Data:
		b, _ := json.Marshal(v.V)
		return string(b)
	default:
		return "ok"
	}
}
