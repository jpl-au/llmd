package main

// This file exposes llmd commands as MCP (Model Context Protocol) tools over
// a stdio transport. AI agents like Claude can connect to "llmd mcp" as an
// MCP server and call tools like "cat", "write", "grep", etc. to read and
// modify documents in the store.
//
// The overall flow:
//   1. Open the store and create a Host (which loads all plugins).
//   2. Walk Host.Commands(), skip any not marked MCP, register the rest as
//      MCP tools with a single generic input schema (args + content).
//   3. Block on server.Run reading JSON-RPC from stdin, writing to stdout.
//
// Every tool uses the same input shape (toolInput) because plugins already
// parse their own flags from args — there's no benefit to per-tool schemas.
// The Content field is piped to the command as stdin, which is how "write"
// and "edit" receive document bodies.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolInput is the MCP input schema shared by all tools.
// Args are passed directly to the plugin's Exec as command-line arguments.
// Content, when non-empty, is delivered as stdin (used by write, edit).
type toolInput struct {
	Args    []string `json:"args"    jsonschema:"description=command arguments"`
	Content string   `json:"content" jsonschema:"description=document content (for write/edit)"`
}

// runMCP starts an MCP server on stdin/stdout and blocks until the
// connection closes (client disconnects or stdin reaches EOF).
// Returns 0 on clean shutdown, 1 on error.
func runMCP() int {
	store, err := llmd.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer store.Close()

	h := host.New(store)
	author := loadConfig()["author"]

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "llmd",
		Version: "dev",
	}, nil)

	for _, cmd := range h.Commands() {
		if !cmd.MCP {
			continue
		}
		registerTool(server, cmd, h, author)
	}

	// Blocks until the client disconnects or stdin closes.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
		return 1
	}
	return 0
}

// registerTool adds a single llmd command as an MCP tool.
// The tool name is cmd.MCPName if set (to avoid collisions like "grep"),
// otherwise cmd.Name. The description includes usage and flag docs so the
// LLM knows how to call it.
func registerTool(server *mcp.Server, cmd *sdk.Command, h *host.Host, author string) {
	name := cmd.Name
	if cmd.MCPName != "" {
		name = cmd.MCPName
	}

	desc := cmd.Desc
	if cmd.Usage != "" {
		desc += "\n\nUsage: llmd " + cmd.Usage
	}
	if len(cmd.Flags) > 0 {
		desc += "\n\nFlags:"
		for _, f := range cmd.Flags {
			if f.Short != "" {
				desc += fmt.Sprintf("\n  -%s, --%s  %s", f.Short, f.Name, f.Desc)
			} else {
				desc += fmt.Sprintf("\n  --%s  %s", f.Name, f.Desc)
			}
		}
	}

	cmdName := cmd.Name // capture for closure

	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: desc,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input toolInput) (*mcp.CallToolResult, any, error) {
		var stdin []byte
		if input.Content != "" {
			stdin = []byte(input.Content)
		}

		result, err := h.Exec(cmdName, input.Args, author, stdin)
		if err != nil {
			// Return command failures as tool errors (IsError: true), not
			// protocol errors. The MCP client sees the error message in
			// Content and can decide how to proceed.
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

// responseText converts an sdk.Response to a plain text string for the MCP
// TextContent response. For Result types that have both Text and Data, we
// prefer Text (human-readable). Marshal errors are ignored because the data
// originated from our own plugins and is always serializable.
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
