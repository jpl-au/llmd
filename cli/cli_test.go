package cli

// cli_test.go is the shared harness for command tests. Every test
// dispatches through the host exactly as the binary does (cli.Exec
// routes the command, the command parses its own flags, the host
// supplies store bridges bound to the test context), so the thin CLI
// layer is exercised end to end without spawning a process.

import (
	"encoding/json"
	"testing"

	"github.com/jpl-au/llmd/host"
	"github.com/jpl-au/llmd/sdk"
)

// newCLI initialises an isolated on-disk store and returns the
// sdk.Context a command receives, acting as author. Use
// host.TestSetup(t, host.TestWorkdir) directly for commands that read
// the working directory (init, config git).
func newCLI(t *testing.T, author string) sdk.Context {
	t.Helper()
	return host.TestSetup(t, host.TestIsolated).Context(t.Context(), author)
}

// run dispatches cmd through the host as the binary would and fails
// the test on error. stdin is the piped input, or nil.
func run(t *testing.T, c sdk.Context, cmd string, stdin []byte, args ...string) sdk.Response {
	t.Helper()
	resp, err := c.Runtime.Dispatch(t.Context(), cmd, args, c.Author, stdin, "")
	if err != nil {
		t.Fatalf("%s %v: %v", cmd, args, err)
	}
	return resp
}

// runErr dispatches cmd through the host and returns the error.
func runErr(t *testing.T, c sdk.Context, cmd string, stdin []byte, args ...string) error {
	t.Helper()
	_, err := c.Runtime.Dispatch(t.Context(), cmd, args, c.Author, stdin, "")
	return err
}

// writeDoc stores content at path as the context's author.
func writeDoc(t *testing.T, c sdk.Context, path, content string) {
	t.Helper()
	run(t, c, "write", []byte(content), path)
}

// text returns the human-readable form of a response: what a terminal
// user sees before any markdown rendering.
func text(r sdk.Response) string {
	switch v := r.(type) {
	case sdk.Text:
		return string(v)
	case sdk.Markdown:
		return v.Text
	case sdk.Result:
		return v.Text
	case sdk.Data:
		b, _ := json.Marshal(v.V)
		return string(b)
	default:
		return ""
	}
}

// data returns the structured payload of a response: what --json, MCP
// and HTTP consumers receive. Nil for plain text.
func data(r sdk.Response) any {
	switch v := r.(type) {
	case sdk.Markdown:
		return v.Data
	case sdk.Result:
		return v.Data
	case sdk.Data:
		return v.V
	default:
		return nil
	}
}
