// mcp.go exposes llmd commands as MCP (Model Context Protocol) tools over
// a stdio transport. AI agents like Claude connect to "llmd mcp" as an
// MCP server and call tools like "cat", "write", "grep", etc.
//
// The flow:
//  1. Walk the host's command table, skip any not marked MCP, register
//     the rest as MCP tools with a single generic input schema (args +
//     content).
//  2. Block on server.Run reading JSON-RPC from stdin, writing to stdout.
//
// Every tool uses the same input shape (toolInput) because extensions
// already parse their own flags from args - there's no benefit to per-tool
// schema. The Content field is piped to the command as stdin, which is how
// "write" and "edit" receive document bodies.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
// Args are passed directly to the extension's Exec as command-line arguments.
// Content, when non-empty, is delivered as stdin (used by write, edit).
// Author identifies who is making the change - the LLM or agent should
// supply this so mutations are attributed correctly. The schema is
// repeated for every tool in the list an agent reads each session, so
// the fields carry no descriptions here: the server instructions
// explain the three of them once.
type toolInput struct {
	Args    []string `json:"args,omitempty"`
	Content string   `json:"content,omitempty"`
	Author  string   `json:"author,omitempty"`
}

// mcpInstructions is what an MCP client shows its model once, when the
// session starts. It carries the conventions every call relies on and
// the working loop for tasks; the tool list carries the rest, and
// guide has the detail.
const mcpInstructions = `llmd is this project's document store and task board, shared by humans and agents.

Calls: every tool takes {args: [...] as on the command line, content: stdin text, author: your name}. Pass author on every call that changes something; reads need none. Never change the configured author.

Documents: paths like notes/plan, no leading slash. write replaces a document (content), edit does search-and-replace, sed substitutes, cat reads, llmd_grep searches. Every write is a new version.

Tasks: "task next" claims the first ready task of up-next (moves it to in-progress, assigns it to you) and returns its spec. Do the work, then "task move <id> review" or "task finish <id>". Raise problems with "audit add <id> <text>". Tasks flagged human are for the user: leave them alone.

Help: guide for the overview, guide <command> for flags and examples.`

// newMCPServer builds the server mcpCmd runs: every command marked MCP
// as a tool, under its MCPName when set, plus the session
// instructions. author is the fallback for calls that name none.
func newMCPServer(rt sdk.Runtime, author string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "llmd",
		Version: app.Tag,
	}, &mcp.ServerOptions{Instructions: mcpInstructions})

	for _, cmd := range rt.Commands() {
		if !cmd.MCP {
			continue
		}
		registerTool(server, rt, cmd, author)
	}
	return server
}

// mcpCmd starts an MCP server on stdin/stdout and blocks until the
// connection closes (client disconnects or stdin reaches EOF).
func mcpCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	author := ctx.Author
	server := newMCPServer(ctx.Runtime, author)

	telemetry.Emit(telemetry.Entry{
		Source:  "mcp",
		Event:   "start",
		Command: "mcp",
		Success: true,
	})

	// Run on the command's context so cancellation from main (Ctrl+C)
	// shuts the server down.
	err := server.Run(ctx, &mcp.StdioTransport{})

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
// otherwise cmd.Name.
func registerTool(server *mcp.Server, rt sdk.Runtime, cmd *sdk.Command, author string) {
	name := cmd.Name
	if cmd.MCPName != "" {
		name = cmd.MCPName
	}
	cmdName := cmd.Name

	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Description: cmd.AgentHelp(),
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

		result, err := rt.Dispatch(ctx, cmdName, input.Args, a, stdin, "")
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
// Markdown responses always emit raw markdown source - the agent on the
// other end of an MCP call wants the unrendered content it can reprocess,
// never glamour escape codes.
func responseText(r sdk.Response) string {
	switch v := r.(type) {
	case sdk.Text:
		return string(v)
	case sdk.Markdown:
		if v.Text != "" {
			return v.Text
		}
		b, err := json.Marshal(v.Data)
		if err != nil {
			slog.Debug("marshalling markdown data", "error", err)
			return "ok"
		}
		return string(b)
	case sdk.Result:
		if v.Text != "" {
			if v.Note != "" {
				return v.Text + "\n" + v.Note
			}
			return v.Text
		}
		b, err := json.Marshal(v.Data)
		if err != nil {
			slog.Debug("marshalling result data", "error", err)
			return "ok"
		}
		return string(b)
	case sdk.Data:
		b, err := json.Marshal(v.V)
		if err != nil {
			slog.Debug("marshalling data response", "error", err)
			return "ok"
		}
		return string(b)
	default:
		return "ok"
	}
}
