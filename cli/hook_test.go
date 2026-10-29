package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/host"
	"github.com/jpl-au/llmd/sdk"
)

// hookSetup gives hook init a working directory of its own and an
// empty HOME, so config.Load sees only the .llmd/config.yaml the test
// writes (if any) and never the developer's global config.
func hookSetup(t *testing.T, localConfig string) sdk.Context {
	t.Helper()
	host.TestHome(t)
	h := host.TestSetup(t, host.TestWorkdir)
	if localConfig != "" {
		path := filepath.Join(h.Dir(), ".llmd", "config.yaml")
		if err := os.WriteFile(path, []byte(localConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return h.Context(t.Context(), "")
}

// TestHookInit covers the generated configuration for each platform:
// every platform runs the same "llmd hook" command, the author comes
// from the platform name or the configured author, and the argument
// errors.
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
			name: "claude runs the hook command",
			args: []string{"init", "claude"},
			wantText: []string{
				`"SessionStart"`, `"SessionEnd"`, `"TaskCompleted"`, `"PostToolUse"`,
				`"matcher": "mcp__llmd.*"`, `"type": "command"`,
				`"command": "llmd --author 'claude' hook"`,
			},
			wantAbsent: []string{"http"},
			wantJSON:   true,
		},
		{
			name:     "claude-code matches claude",
			args:     []string{"init", "claude-code"},
			wantText: []string{`"PostToolUse"`, `"command": "llmd --author 'claude-code' hook"`},
			wantJSON: true,
		},
		{
			name: "gemini runs the hook command",
			args: []string{"init", "gemini"},
			wantText: []string{
				`"event": "SessionStart"`, `"event": "AfterTool"`, `"type": "command"`,
				`"command": "llmd --author 'gemini' hook"`,
			},
			wantAbsent: []string{"queue", "http"},
			wantJSON:   true,
		},
		{
			name: "generic is a wrapper script around the hook command",
			args: []string{"init", "generic"},
			wantText: []string{
				"#!/bin/sh", `llmd --author 'generic' hook --task "$TASK_ID"`,
				"emit session.start", "emit task.completed", "emit task.failed", "exit $EXIT",
			},
			wantAbsent: []string{"queue send", "task move"},
		},
		{
			name:     "unknown platform falls back to generic",
			args:     []string{"init", "my-agent"},
			wantText: []string{"#!/bin/sh", `llmd --author 'my-agent' hook`},
		},
		{
			name:       "configured author is used",
			config:     "author: jake\n",
			args:       []string{"init", "claude"},
			wantText:   []string{`"command": "llmd --author 'jake' hook"`},
			wantAbsent: []string{"'claude'"},
			wantJSON:   true,
		},
		{
			name:     "configured author applies to the generic script",
			config:   "author: jake\n",
			args:     []string{"init", "generic"},
			wantText: []string{`llmd --author 'jake' hook`},
		},
		{
			name:       "malformed config falls back to the platform name",
			config:     "author: [unterminated\n",
			args:       []string{"init", "claude"},
			wantText:   []string{`"command": "llmd --author 'claude' hook"`},
			wantAbsent: []string{"jake"},
			wantJSON:   true,
		},
		{name: "missing platform", args: []string{"init"}, wantErr: sdk.ErrMissingArg, wantMsg: "claude, gemini, generic"},
		{name: "unknown subcommand", args: []string{"install", "claude"}, wantErr: sdk.ErrInvalidArg, wantMsg: "unknown subcommand"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := hookSetup(t, tt.config)

			if tt.wantErr != nil {
				err := runErr(t, c, "hook", nil, tt.args...)
				if !errors.Is(err, tt.wantErr) {
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

// TestHook covers handling a hook payload: each event's actions, the
// platform chosen from the author, the --task flag, and the argument
// errors. {key} in a payload or in args is the key of a task seeded
// with a spec so it can leave the backlog.
func TestHook(t *testing.T) {
	tests := []struct {
		name        string
		author      string
		limit       int // hook.limit; 0 is the test default of showing everything
		seed        func(t *testing.T, c sdk.Context)
		args        []string
		payload     string
		wantEvent   string
		wantActions []string
		wantText    []string // substrings of the response text
		wantAbsent  []string // substrings the response text must not have
		wantStatus  string   // task status afterwards, when set
		wantMessage string   // a queue message every consumer can see afterwards
		wantAudit   string   // audit content recorded against the task
		wantErr     error
	}{
		{
			name:   "claude session start lists pending messages",
			author: "claude",
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "queue", nil, "send", "hello hook", "--assign", "claude")
			},
			payload:     `{"hook_event_name":"SessionStart","session_id":"s1"}`,
			wantEvent:   "session.start",
			wantActions: []string{"queue.ls"},
			wantText:    []string{"hello hook"},
		},
		{
			name:   "session start counts a long backlog and shows the newest",
			author: "claude",
			limit:  2,
			seed: func(t *testing.T, c sdk.Context) {
				for _, m := range []string{"first", "second", "third", "fourth"} {
					run(t, c, "queue", nil, "send", m, "--assign", "claude")
				}
			},
			payload:     `{"hook_event_name":"SessionStart","session_id":"s1"}`,
			wantEvent:   "session.start",
			wantActions: []string{"queue.ls"},
			wantText:    []string{"pending queue messages, newest 2 shown", "third", "fourth"},
			wantAbsent:  []string{"first", "second"},
		},
		{
			name:        "claude session end sends a message",
			author:      "claude",
			payload:     `{"hook_event_name":"SessionEnd"}`,
			wantEvent:   "session.end",
			wantActions: []string{"queue.send"},
			wantMessage: "Session ended",
		},
		{
			name:        "stop with --task names the task",
			author:      "claude-code",
			args:        []string{"--task", "{key}"},
			payload:     `{"hook_event_name":"Stop"}`,
			wantEvent:   "session.stop",
			wantActions: []string{"queue.send"},
			wantMessage: "Session ended for task {key}",
		},
		{
			name:        "claude task completed finishes the task",
			author:      "claude",
			args:        []string{"--task", "{key}"},
			payload:     `{"hook_event_name":"TaskCompleted"}`,
			wantEvent:   "task.completed",
			wantActions: []string{"task.finish", "queue.send"},
			wantStatus:  "done",
			wantMessage: "Completed for task {key}",
		},
		{
			name:        "generic task failed blocks the task",
			author:      "my-agent",
			payload:     `{"event":"task.failed","task_id":"{key}","content":"tests broke"}`,
			wantEvent:   "task.failed",
			wantActions: []string{"task.move", "queue.send"},
			wantStatus:  "blocked",
			wantMessage: "tests broke",
		},
		{
			name:        "generic tool use records an audit",
			author:      "my-agent",
			payload:     `{"event":"tool.post","task_id":"{key}","content":"ran the tests"}`,
			wantEvent:   "tool.post",
			wantActions: []string{"audit.add"},
			wantAudit:   "ran the tests",
		},
		{
			name:      "gemini after tool without a task does nothing",
			author:    "gemini",
			payload:   `{"event":"AfterTool"}`,
			wantEvent: "tool.post",
		},
		{
			name:      "unknown event is accepted",
			author:    "my-agent",
			payload:   `{"event":"something.else"}`,
			wantEvent: "something.else",
		},
		{name: "malformed payload", author: "claude", payload: `{not json`, wantErr: sdk.ErrInvalidArg},
		{name: "empty payload", author: "claude", payload: "", wantErr: sdk.ErrMissingArg},
		{name: "no author", author: "", payload: `{"event":"session.start"}`, wantErr: sdk.ErrMissingArg},
		{name: "unknown flag", author: "claude", args: []string{"--bogus"}, payload: `{}`, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := host.TestSetup(t, host.TestIsolated, host.WithHookLimit(tt.limit)).Context(t.Context(), tt.author)
			seedCtx := c
			if tt.author == "" {
				seedCtx = newCLI(t, "alice")
			}
			task := addTask(t, seedCtx, "Hook task")
			if tt.seed != nil {
				tt.seed(t, c)
			}
			expand := func(s string) string { return strings.ReplaceAll(s, "{key}", task.Key) }
			var args []string
			for _, a := range tt.args {
				args = append(args, expand(a))
			}
			var stdin []byte
			if tt.payload != "" {
				stdin = []byte(expand(tt.payload))
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "hook", stdin, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("hook %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, c, "hook", stdin, args...)
			result, ok := data(resp).(*sdk.HookResult)
			if !ok {
				t.Fatalf("hook Data = %T, want *sdk.HookResult", data(resp))
			}
			if result.Event != tt.wantEvent {
				t.Errorf("hook event = %q, want %q", result.Event, tt.wantEvent)
			}
			if !slices.Equal(result.Actions, tt.wantActions) {
				t.Errorf("hook actions = %v, want %v", result.Actions, tt.wantActions)
			}
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("hook text = %q, want to contain %q", got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("hook text = %q, must not contain %q", got, absent)
				}
			}
			if tt.wantStatus != "" {
				if st := taskRead(t, c, task.Key).Status; st != tt.wantStatus {
					t.Errorf("task status after hook = %q, want %q", st, tt.wantStatus)
				}
			}
			if tt.wantMessage != "" {
				msgs, err := c.Queue.Pending("observer", 0)
				if err != nil {
					t.Fatalf("Pending: %v", err)
				}
				found := false
				for _, m := range msgs {
					if m.Payload == expand(tt.wantMessage) {
						found = true
					}
				}
				if !found {
					t.Errorf("queue has no message %q after hook; messages: %+v", expand(tt.wantMessage), msgs)
				}
			}
			if tt.wantAudit != "" {
				audits, err := c.Audits.List(sdk.AuditListOpts{})
				if err != nil {
					t.Fatalf("Audits.List: %v", err)
				}
				if len(audits) != 1 || audits[0].Content != tt.wantAudit || audits[0].Target != task.Key {
					t.Errorf("audits after hook = %+v, want one on %s with %q", audits, task.Key, tt.wantAudit)
				}
			}
		})
	}
}
