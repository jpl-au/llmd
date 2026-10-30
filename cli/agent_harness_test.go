package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestAgentHarness covers showing the harness an agent runs under,
// naming one, and the errors for a missing argument, a missing author,
// an unknown agent and a harness this build does not have.
func TestAgentHarness(t *testing.T) {
	tests := []struct {
		name     string
		author   string
		args     []string
		wantText string
		wantErr  error
		wantMsg  string
	}{
		{name: "shows llmd's own by default", args: []string{"claude-code"}, wantText: "claude-code runs under the harness llmd."},
		{name: "names a harness", author: "alice", args: []string{"claude-code", "llmd"}, wantText: "claude-code runs under the harness llmd."},
		{name: "naming needs an author", args: []string{"claude-code", "llmd"}, wantErr: sdk.ErrMissingArg, wantMsg: "author is required"},
		{name: "an agent is required", wantErr: sdk.ErrMissingArg, wantMsg: "an agent name is required"},
		{name: "unknown agent", args: []string{"ghost"}, wantErr: sdk.ErrNotFound},
		{name: "unknown harness", author: "alice", args: []string{"claude-code", "missing"}, wantErr: sdk.ErrNotFound, wantMsg: "no harness missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := agentSetup(t, tt.author)
			run(t, c, "agent", nil, "add", "claude-code")

			args := append([]string{"harness"}, tt.args...)
			if tt.wantErr != nil {
				err := runErr(t, c, "agent", nil, args...)
				if !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("agent %v error = %v, want %v containing %q", args, err, tt.wantErr, tt.wantMsg)
				}
				return
			}
			resp := run(t, c, "agent", nil, args...)
			if got := text(resp); got != tt.wantText {
				t.Errorf("agent %v = %q, want %q", args, got, tt.wantText)
			}
			if got := data(resp).(harnessView); got.Agent != "claude-code" || got.Harness != "llmd" {
				t.Errorf("agent %v data = %+v, want claude-code under llmd", args, got)
			}
		})
	}
}
