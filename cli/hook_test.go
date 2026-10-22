package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// hookSetup gives hook init a working directory of its own and an
// empty HOME, so config.Load sees only the .llmd/config.yaml the test
// writes (if any) and never the developer's global config.
func hookSetup(t *testing.T, localConfig string) sdk.Context {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	h := host.TestSetup(t, host.TestWorkdir)
	if localConfig != "" {
		path := filepath.Join(h.Dir(), ".llmd", "config.yaml")
		if err := os.WriteFile(path, []byte(localConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return h.Context(t.Context(), "")
}

// TestHookInit covers the generated configuration for each platform,
// substring matching of the platform name, the config-driven server
// address and author, and the argument errors. hook init does not read
// stdin: it prints a configuration for the agent platform to install.
func TestHookInit(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		args       []string
		wantText   []string
		wantAbsent []string
		wantJSON   bool
		wantErr    error
		wantMsg    string
	}{
		{
			name: "claude posts to the hook endpoint",
			args: []string{"init", "claude"},
			wantText: []string{
				`"SessionStart"`, `"SessionEnd"`, `"TaskCompleted"`, `"PostToolUse"`,
				`"matcher": "mcp__llmd.*"`, `"type": "http"`,
				`"url": "http://localhost:5563/hook"`, `"headers": {"Author": "claude"}`,
			},
			wantJSON: true,
		},
		{
			name:     "claude-code matches claude",
			args:     []string{"init", "claude-code"},
			wantText: []string{`"PostToolUse"`, `"headers": {"Author": "claude-code"}`},
			wantJSON: true,
		},
		{
			name: "gemini polls the queue",
			args: []string{"init", "gemini"},
			wantText: []string{
				`"event": "SessionStart"`, `"event": "AfterTool"`, `"type": "command"`,
				"llmd --author gemini queue ls --limit 5", "llmd --author gemini queue peek",
			},
			wantAbsent: []string{"http://"},
			wantJSON:   true,
		},
		{
			name: "generic is a wrapper script",
			args: []string{"init", "generic"},
			wantText: []string{
				"#!/bin/bash", `llmd --author "generic" task move "$TASK_ID" in-progress`,
				`llmd --author "generic" queue send "Task $TASK_ID completed"`,
				`llmd --author "generic" task move "$TASK_ID" blocked`, "exit $EXIT",
			},
		},
		{
			name:     "unknown platform falls back to generic",
			args:     []string{"init", "my-agent"},
			wantText: []string{"#!/bin/bash", `llmd --author "my-agent" queue send`},
		},
		{
			name:   "config author and server address",
			config: "author: jake\nserver:\n  addr: \"localhost:9090\"\n",
			args:   []string{"init", "claude"},
			wantText: []string{
				`"url": "http://localhost:9090/hook"`, `"headers": {"Author": "jake"}`,
			},
			wantAbsent: []string{"5563", `"Author": "claude"`},
			wantJSON:   true,
		},
		{
			name:     "config author applies to the generic script",
			config:   "author: jake\n",
			args:     []string{"init", "generic"},
			wantText: []string{`llmd --author "jake" task move`},
		},
		{
			name:       "malformed config falls back to defaults",
			config:     "author: [unterminated\n",
			args:       []string{"init", "claude"},
			wantText:   []string{`"url": "http://localhost:5563/hook"`, `"headers": {"Author": "claude"}`},
			wantAbsent: []string{"jake"},
			wantJSON:   true,
		},
		{name: "missing platform", args: []string{"init"}, wantErr: sdk.ErrMissingArg, wantMsg: "claude, gemini, generic"},
		{name: "missing subcommand", args: nil, wantErr: sdk.ErrMissingArg},
		{name: "unknown subcommand", args: []string{"install", "claude"}, wantMsg: "unknown subcommand"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := hookSetup(t, tt.config)

			if tt.wantErr != nil || tt.wantMsg != "" {
				err := runErr(t, c, "hook", nil, tt.args...)
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("hook %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
					t.Errorf("hook %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
				}
				return
			}

			resp := run(t, c, "hook", nil, tt.args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("hook %v response type = %T, want sdk.Text", tt.args, resp)
			}
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("hook %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("hook %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if tt.wantJSON && !json.Valid([]byte(got)) {
				t.Errorf("hook %v output is not valid JSON:\n%s", tt.args, got)
			}
			if !strings.HasSuffix(got, "\n") {
				t.Errorf("hook %v output does not end with a newline", tt.args)
			}
		})
	}
}
