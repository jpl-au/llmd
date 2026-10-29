package cli

import (
	"errors"
	"runtime"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/sdk"
)

// TestAgentRunExitCode checks that agent run reports the agent's exit
// code as an error rather than ending the process: it once called
// os.Exit from the handler, which skipped closing the store and would
// have ended an MCP or HTTP server that dispatched it.
func TestAgentRunExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	tests := []struct {
		name     string
		script   string
		wantCode int
	}{
		{name: "success", script: "exit 0"},
		{name: "failure", script: "exit 3", wantCode: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			worktree := t.TempDir()
			if err := agents.WriteRunConfig(worktree, agents.RunConfig{
				TaskKey: "abcdefghij", Agent: "test", Command: "sh", Args: []string{"-c", tt.script},
			}); err != nil {
				t.Fatal(err)
			}

			err := runErr(t, c, "agent", nil, "run", "abcdefghij", "--worktree", worktree)

			var exit *sdk.ExitError
			switch {
			case tt.wantCode == 0 && err != nil:
				t.Fatalf("agent run error = %v, want nil", err)
			case tt.wantCode != 0 && (!errors.As(err, &exit) || exit.Code != tt.wantCode):
				t.Fatalf("agent run error = %v, want ExitError with code %d", err, tt.wantCode)
			}
			// The handler returned, so the store is still usable.
			writeDoc(t, c, "after", "still open")
		})
	}
}
