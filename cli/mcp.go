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
// parse their own flags from args — there's no benefit to per-tool schemas.
// The Content field is piped to the command as stdin, which is how "write"
// and "edit" receive document bodies.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

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
// Author identifies who is making the change — the LLM or agent should
// supply this so mutations are attributed correctly.
type toolInput struct {
	Args    []string `json:"args"    jsonschema:"command arguments"`
	Content string   `json:"content" jsonschema:"document content (for write/edit)"`
	Author  string   `json:"author"  jsonschema:"author name for attributing changes"`
}

// mcpCmd starts an MCP server on stdin/stdout and blocks until the
// connection closes (client disconnects or stdin reaches EOF).
// diagWriter intercepts writes to stdout and logs each line to a
// diagnostic file with a timestamp, so we can see exactly what the
// MCP server sends and when.
type diagWriter struct {
	real io.Writer // original os.Stdout
	log  io.Writer // diagnostic log file
	mu   sync.Mutex
}

func (d *diagWriter) Write(p []byte) (int, error) {
	d.mu.Lock()
	ts := time.Now().Format("15:04:05.000")
	// Log each line separately for readability.
	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		line := scanner.Text()
		// Try to extract the method from JSON-RPC messages.
		var msg struct {
			Method string `json:"method"`
			ID     any    `json:"id"`
		}
		label := line
		if json.Unmarshal([]byte(line), &msg) == nil && msg.Method != "" {
			if msg.ID != nil {
				label = fmt.Sprintf("response/request method=%s", msg.Method)
			} else {
				label = fmt.Sprintf("notification method=%s", msg.Method)
			}
		}
		fmt.Fprintf(d.log, "%s  OUT  %s\n", ts, label)
	}
	d.mu.Unlock()
	return d.real.Write(p)
}

func mcpCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	author := ctx.Author

	// Set up diagnostic logging to /tmp/llmd-mcp-diag.log.
	diagFile, err := os.OpenFile("/tmp/llmd-mcp-diag.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		slog.Warn("mcp: could not open diagnostic log", "err", err)
	}
	if diagFile != nil {
		defer diagFile.Close()
		fmt.Fprintf(diagFile, "%s  START  mcpCmd entered\n", time.Now().Format("15:04:05.000"))

		// Intercept stdin so we can log what the client sends.
		realStdin := os.Stdin
		stdinPR, stdinPW, _ := os.Pipe()
		os.Stdin = stdinPR
		go func() {
			buf := make([]byte, 64*1024)
			for {
				n, err := realStdin.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					ts := time.Now().Format("15:04:05.000")
					fmt.Fprintf(diagFile, "%s  IN   %s\n", ts, strings.TrimRight(chunk, "\r\n"))
					stdinPW.Write(buf[:n])
				}
				if err != nil {
					stdinPW.Close()
					return
				}
			}
		}()

		// Replace os.Stdout so the SDK's StdioTransport writes through
		// our interceptor.
		realStdout := os.Stdout
		pr, pw, _ := os.Pipe()
		os.Stdout = pw

		dw := &diagWriter{real: realStdout, log: diagFile}
		go func() {
			buf := make([]byte, 64*1024)
			for {
				n, err := pr.Read(buf)
				if n > 0 {
					dw.Write(buf[:n])
				}
				if err != nil {
					return
				}
			}
		}()
		defer func() {
			pw.Close()
			os.Stdout = realStdout
		}()
	}

	diag := func(msg string) {
		if diagFile != nil {
			fmt.Fprintf(diagFile, "%s  EVENT  %s\n", time.Now().Format("15:04:05.000"), msg)
		}
	}

	// ready is closed when the client completes the MCP handshake
	// (sends initialized notification). Nothing should send
	// notifications before this.
	ready := make(chan struct{})

	diag("BEFORE mcp.NewServer")
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "llmd",
		Version: app.Tag,
	}, &mcp.ServerOptions{
		// Disable automatic list_changed notifications — we control
		// when notifications fire, not the SDK.
		Capabilities: &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{},
			Resources: &mcp.ResourceCapabilities{},
		},
		InitializedHandler: func(_ context.Context, _ *mcp.InitializedRequest) {
			diag("CALLBACK InitializedHandler — client handshake complete")
			close(ready)
		},
	})
	diag("AFTER mcp.NewServer")

	// Test resource: llmd://status. Returns a timestamp so the client
	// can verify the server is alive.
	const statusURI = "llmd://status"
	diag("BEFORE server.AddResource llmd://status")
	server.AddResource(&mcp.Resource{
		URI:         statusURI,
		Name:        "llmd status",
		Description: "Store status — subscribe to receive update notifications",
		MIMEType:    "text/plain",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		diag("CALLBACK ReadResource called for llmd://status")
		msg := fmt.Sprintf("llmd status checked at %s", time.Now().Format(time.RFC3339))
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      statusURI,
				MIMEType: "text/plain",
				Text:     msg,
			}},
		}, nil
	})
	diag("AFTER server.AddResource llmd://status")

	// Simulate event bus: fire a ResourceUpdated notification every
	// 10 seconds, but only AFTER the handshake completes.
	go func() {
		diag("ticker goroutine: waiting for handshake")
		<-ready
		diag("ticker goroutine: handshake done, starting 10s ticker")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			diag("BEFORE server.ResourceUpdated (ticker)")
			if err := server.ResourceUpdated(context.Background(), &mcp.ResourceUpdatedNotificationParams{
				URI: statusURI,
			}); err != nil {
				diag(fmt.Sprintf("AFTER server.ResourceUpdated (ticker) — FAILED: %v", err))
				slog.Debug("mcp: resource updated notification failed", "err", err)
			} else {
				diag("AFTER server.ResourceUpdated (ticker) — ok")
			}
		}
	}()

	diag("BEFORE registering tools")
	toolCount := 0
	for _, cmd := range sdk.AllCommands() {
		if !cmd.MCP {
			continue
		}
		name := cmd.Name
		if cmd.MCPName != "" {
			name = cmd.MCPName
		}
		diag(fmt.Sprintf("BEFORE mcp.AddTool %s", name))
		registerTool(server, cmd, author)
		diag(fmt.Sprintf("AFTER mcp.AddTool %s", name))
		toolCount++
	}
	diag(fmt.Sprintf("AFTER registering all %d tools", toolCount))

	diag("BEFORE server.Run (blocking on stdio)")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		diag(fmt.Sprintf("AFTER server.Run — FAILED: %v", err))
		return nil, fmt.Errorf("mcp: %w", err)
	}
	diag("AFTER server.Run — returned cleanly")
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
		// MCP callers are non-interactive — no config author fallback.
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
