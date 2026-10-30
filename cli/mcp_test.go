package cli

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpSession registers the named commands (every MCP command when names
// is empty) on a fresh server bound to c's runtime, exactly as mcpCmd
// does, and returns a client session connected over in-memory
// transports. author is the server-level fallback author.
func mcpSession(t *testing.T, c sdk.Context, author string, names ...string) *mcp.ClientSession {
	t.Helper()
	var server *mcp.Server
	if len(names) == 0 {
		server = newMCPServer(c.Runtime, author)
	} else {
		server = mcp.NewServer(&mcp.Implementation{Name: "llmd-test", Version: "test"}, &mcp.ServerOptions{Instructions: mcpInstructions})
	}
	cmds := c.Runtime.Commands()
	for _, name := range names {
		cmd, ok := cmds[name]
		if !ok {
			t.Fatalf("no command %q registered", name)
		}
		registerTool(server, c.Runtime, cmd, author)
	}

	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	cs, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() {
		if err := cs.Close(); err != nil {
			t.Errorf("closing client session: %v", err)
		}
		if err := ss.Wait(); err != nil {
			t.Logf("server session: %v", err)
		}
	})
	return cs
}

// mcpToolNames lists the tool names a session exposes.
func mcpToolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	return names
}

// mcpCall invokes tool with the shared input shape and returns the
// text of the first content block plus the IsError flag.
func mcpCall(t *testing.T, cs *mcp.ClientSession, tool string, input map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: tool, Arguments: input})
	if err != nil {
		t.Fatalf("CallTool(%s, %v): %v", tool, input, err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool(%s) returned %d content blocks, want 1", tool, len(res.Content))
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s) content = %T, want *mcp.TextContent", tool, res.Content[0])
	}
	return tc.Text, res.IsError
}

// TestMCPRegisterTool covers what the MCP server exposes: only commands
// marked MCP, under their MCPName when set, with a description that
// carries the summary, the usage line, every flag and a pointer to the
// guide, and with the session instructions.
func TestMCPRegisterTool(t *testing.T) {
	c := newCLI(t, "")
	cs := mcpSession(t, c, "")
	names := mcpToolNames(t, cs)

	if got := cs.InitializeResult().Instructions; got != mcpInstructions || !strings.Contains(got, "author") {
		t.Errorf("Instructions = %q, want mcpInstructions mentioning author", got)
	}

	for _, want := range []string{"cat", "write", "audit", "queue", "agent", "task", "llmd_grep", "llmd_find", "llmd_glob", "llmd_init"} {
		if !slices.Contains(names, want) {
			t.Errorf("tools = %v, want to contain %q", names, want)
		}
	}
	for _, absent := range []string{"mcp", "serve", "hook", "version", "config", "grep", "find", "glob", "init"} {
		if slices.Contains(names, absent) {
			t.Errorf("tools = %v, must not contain %q", names, absent)
		}
	}
	// Every MCP command is registered exactly once, nothing else is.
	var wantCount int
	for _, cmd := range c.Runtime.Commands() {
		if cmd.MCP {
			wantCount++
		}
	}
	if len(names) != wantCount {
		t.Errorf("registered %d tools, want %d (one per MCP command)", len(names), wantCount)
	}

	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.InputSchema == nil {
			t.Errorf("%s tool has no input schema", tool.Name)
		}
		switch tool.Name {
		case "queue":
			// A command with an Brief owns its text: no generated flags.
			for _, want := range []string{queueSpec.Brief, "\nUsage: " + sdk.Program() + " queue <subcommand> [options]", "\nDetails: guide queue"} {
				if !strings.Contains(tool.Description, want) {
					t.Errorf("queue tool description = %q, want to contain %q", tool.Description, want)
				}
			}
			for _, absent := range []string{"Subcommands (passed as first arg)", "Flags:", "--limit <int>"} {
				if strings.Contains(tool.Description, absent) {
					t.Errorf("queue tool description = %q, must not contain %q", tool.Description, absent)
				}
			}
		case "history":
			// A command without one gets the first paragraph and its flags.
			for _, want := range []string{historySpec.Summary() + "\n", "\nUsage: " + sdk.Program() + " history", "\nFlags:", "\n  -n, --limit <int>  Maximum versions", "\n  --all  Show every version", "\nDetails: guide history"} {
				if !strings.Contains(tool.Description, want) {
					t.Errorf("history tool description = %q, want to contain %q", tool.Description, want)
				}
			}
			if strings.Contains(tool.Description, "Newest versions are shown first") {
				t.Errorf("history tool description = %q, carries the rest of Desc", tool.Description)
			}
		case "tag":
			// Short flags are listed with their long form.
			if !strings.Contains(tool.Description, "\n  -d, --delete  Remove a tag") {
				t.Errorf("tag tool description = %q, want the short flag form", tool.Description)
			}
		}
	}
}

// TestMCPToolListBudget keeps the tool list, which every session reads
// before its first call, within a byte budget. The list once came to
// 23 KB and is about 12 KB now; the budget leaves room for a command
// or two and catches the descriptions growing back into prose.
func TestMCPToolListBudget(t *testing.T) {
	const budget = 14 * 1024
	c := newCLI(t, "")
	cs := mcpSession(t, c, "")
	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > budget {
		t.Errorf("tool list is %d bytes, want at most %d", len(b), budget)
	}
	for _, tool := range res.Tools {
		if !strings.Contains(tool.Description, "Details: guide "+strings.TrimPrefix(tool.Name, "llmd_")) {
			t.Errorf("%s description does not point at its guide: %q", tool.Name, tool.Description)
		}
	}
}

// TestMCPCallTool covers tool calls end to end over the protocol: args
// and content reach the command, the author on the call attributes the
// change, an empty author falls back to the server-level author, and
// failures come back as IsError text rather than protocol errors.
//
// Every call sends all three input fields: the generated schema marks
// them required, so a missing key is rejected before the tool runs
// (see TestMCPOptionalInput for the documented contract).
func TestMCPCallTool(t *testing.T) {
	tests := []struct {
		name         string
		serverAuthor string
		tool         string
		input        map[string]any
		wantText     []string
		wantAuthor   string // author recorded on docs/x, when set
		wantErr      bool
	}{
		{
			name:       "write with content and author",
			tool:       "write",
			input:      map[string]any{"args": []string{"docs/x"}, "content": "# From MCP\n\nbody", "author": "bob"},
			wantText:   []string{"docs/x"},
			wantAuthor: "bob",
		},
		{
			name:         "empty author falls back to the server author",
			serverAuthor: "alice",
			tool:         "write",
			input:        map[string]any{"args": []string{"docs/x"}, "content": "# From MCP", "author": ""},
			wantAuthor:   "alice",
		},
		{
			name:     "no author anywhere is rejected for mutations",
			tool:     "write",
			input:    map[string]any{"args": []string{"docs/x"}, "content": "# From MCP", "author": ""},
			wantText: []string{"author"},
			wantErr:  true,
		},
		{
			name:     "read returns raw markdown",
			tool:     "cat",
			input:    map[string]any{"args": []string{"docs/seed"}, "content": "", "author": ""},
			wantText: []string{"# Seed\n\nseed body"},
		},
		{
			name:     "not found is an error result",
			tool:     "cat",
			input:    map[string]any{"args": []string{"docs/missing"}, "content": "", "author": ""},
			wantText: []string{"does not exist."},
			wantErr:  true,
		},
		{
			name:     "missing argument is an error result",
			tool:     "audit",
			input:    map[string]any{"args": []string{}, "content": "", "author": "bob"},
			wantText: []string{"a subcommand is required."},
			wantErr:  true,
		},
		{
			name:     "subcommand through args",
			tool:     "audit",
			input:    map[string]any{"args": []string{"add", "docs/seed", "Looks fine."}, "content": "", "author": "bob"},
			wantText: []string{"Created audit ", " on docs/seed [pending]"},
		},
		{
			name:     "content is delivered as stdin",
			tool:     "audit",
			input:    map[string]any{"args": []string{"add", "docs/seed"}, "content": "From the content field.", "author": "bob"},
			wantText: []string{"Created audit "},
		},
		{
			name:     "queue requires an author",
			tool:     "queue",
			input:    map[string]any{"args": []string{"ls"}, "content": "", "author": ""},
			wantText: []string{"an author is required."},
			wantErr:  true,
		},
		{
			name:         "queue with the server author",
			serverAuthor: "alice",
			tool:         "queue",
			input:        map[string]any{"args": []string{"send", "hello", "--assign", "bob"}, "content": "", "author": ""},
			wantText:     []string{"Sent ", " -> bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "seed")
			writeDoc(t, c, "docs/seed", "# Seed\n\nseed body")
			cs := mcpSession(t, c, tt.serverAuthor, "cat", "write", "audit", "queue")

			got, isErr := mcpCall(t, cs, tt.tool, tt.input)
			if isErr != tt.wantErr {
				t.Errorf("%s %v IsError = %v (%q), want %v", tt.tool, tt.input, isErr, got, tt.wantErr)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("%s %v = %q, want to contain %q", tt.tool, tt.input, got, want)
				}
			}
			if strings.Contains(got, "\x1b[") {
				t.Errorf("%s %v = %q, must not contain terminal escape codes", tt.tool, tt.input, got)
			}
			if tt.wantAuthor == "" {
				return
			}
			versions, err := c.Documents.History("docs/x", 0)
			if err != nil || len(versions) == 0 {
				t.Fatalf("History(docs/x) = %v, %v", versions, err)
			}
			if versions[0].Author != tt.wantAuthor {
				t.Errorf("docs/x written by %q, want %q", versions[0].Author, tt.wantAuthor)
			}
			if got := text(run(t, c, "cat", nil, "docs/x")); !strings.HasPrefix(got, "# From MCP") {
				t.Errorf("cat docs/x = %q, want the content delivered through the tool call", got)
			}
		})
	}

	t.Run("content reaches the command as stdin", func(t *testing.T) {
		c := newCLI(t, "seed")
		cs := mcpSession(t, c, "", "audit")
		got, isErr := mcpCall(t, cs, "audit", map[string]any{"args": []string{"add", "docs/seed"}, "content": "From the content field.", "author": "bob"})
		if isErr {
			t.Fatalf("audit add = %q, want success", got)
		}
		list := auditsOf(t, run(t, c, "audit", nil, "list"))
		if len(list) != 1 || list[0].Content != "From the content field." || list[0].Author != "bob" {
			t.Errorf("audit list after MCP add = %+v, want one audit by bob with the content field as body", list)
		}
	})

	t.Run("unknown tool is a protocol error", func(t *testing.T) {
		c := newCLI(t, "")
		cs := mcpSession(t, c, "", "cat")
		_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "write", Arguments: map[string]any{}})
		if err == nil {
			t.Errorf("CallTool(write) on a server without it error = nil, want an error")
		}
	})

	t.Run("call context cancellation", func(t *testing.T) {
		c := newCLI(t, "")
		cs := mcpSession(t, c, "", "cat")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "cat", Arguments: map[string]any{"args": []string{"x"}}}); err == nil {
			t.Errorf("CallTool with a cancelled context error = nil, want an error")
		}
	})
}

// TestMCPOptionalInput asserts the documented input contract from
// guide/mcp.md: content and author are optional, read-only tools need
// no author, and a call without one falls back to the server-level
// author.
func TestMCPOptionalInput(t *testing.T) {
	c := newCLI(t, "seed")
	writeDoc(t, c, "docs/seed", "# Seed")
	cs := mcpSession(t, c, "alice", "cat", "write")

	res, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"content", "author"} {
			if slices.Contains(parsed.Required, field) {
				t.Errorf("%s input schema requires %q, want it optional", tool.Name, field)
			}
		}
	}

	if got, isErr := mcpCall(t, cs, "cat", map[string]any{"args": []string{"docs/seed"}}); isErr || !strings.Contains(got, "# Seed") {
		t.Errorf("cat with args only = %q (IsError %v), want the document", got, isErr)
	}
	if got, isErr := mcpCall(t, cs, "write", map[string]any{"args": []string{"docs/x"}, "content": "# X"}); isErr {
		t.Errorf("write without author = %q, want the server author to apply", got)
	}
	if versions, err := c.Documents.History("docs/x", 0); err != nil || len(versions) == 0 || versions[0].Author != "alice" {
		t.Errorf("History(docs/x) = %v, %v, want a version by alice", versions, err)
	}
}

// TestMCPResponseText covers the conversion of every response kind to
// the plain text an MCP client receives: text wins when present, data
// is JSON-encoded otherwise, and anything else is "ok".
func TestMCPResponseText(t *testing.T) {
	tests := []struct {
		name string
		resp sdk.Response
		want string
	}{
		{name: "text", resp: sdk.Text("plain"), want: "plain"},
		{name: "markdown text is raw", resp: sdk.Markdown{Text: "# Title\n\n*em*", Data: map[string]int{"n": 1}}, want: "# Title\n\n*em*"},
		{name: "markdown without text encodes data", resp: sdk.Markdown{Data: map[string]int{"n": 1}}, want: `{"n":1}`},
		{name: "result text wins over data", resp: sdk.Result{Text: "Sent abc", Data: map[string]string{"key": "abc"}}, want: "Sent abc"},
		{name: "result without text encodes data", resp: sdk.Result{Data: []string{"a", "b"}}, want: `["a","b"]`},
		{name: "data is encoded", resp: sdk.Data{V: map[string]any{"ok": true}}, want: `{"ok":true}`},
		{name: "nil is ok", resp: nil, want: "ok"},
		{name: "unencodable data falls back to ok", resp: sdk.Data{V: map[string]any{"f": func() {}}}, want: "ok"},
		{name: "unencodable result data falls back to ok", resp: sdk.Result{Data: make(chan int)}, want: "ok"},
		{name: "unencodable markdown data falls back to ok", resp: sdk.Markdown{Data: make(chan int)}, want: "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := responseText(tt.resp); got != tt.want {
				t.Errorf("responseText(%#v) = %q, want %q", tt.resp, got, tt.want)
			}
		})
	}
}
