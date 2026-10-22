package host

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestHooksHandle covers one case per hook event through the host's
// HookStore: the platform chosen from the author, the actions each
// event takes, the task fallback from HookOpts, and the argument
// errors. {key} in a payload is replaced with the key of a task
// seeded with a spec so it can leave the backlog.
func TestHooksHandle(t *testing.T) {
	tests := []struct {
		name        string
		author      string
		seed        func(t *testing.T, c sdk.Context, key string)
		payload     string
		opts        func(key string) sdk.HookOpts
		wantEvent   string
		wantTask    bool // result names the task
		wantActions []string
		wantContent []string // substrings of result.Content
		wantStatus  string   // task status afterwards, when set
		wantMessage string   // a queue payload every consumer can see afterwards
		wantAudit   string   // audit content recorded against the task
		wantErr     error
	}{
		{
			name:   "claude session start lists pending messages",
			author: "claude",
			seed: func(t *testing.T, c sdk.Context, _ string) {
				sendMsg(t, c, sdk.SendOpts{Payload: "hello hook", AssignedTo: "claude"})
				sendMsg(t, c, sdk.SendOpts{Payload: "not for you", AssignedTo: "gemini"})
			},
			payload:     `{"hook_event_name":"SessionStart","session_id":"s1"}`,
			wantEvent:   "session.start",
			wantActions: []string{"queue.ls"},
			wantContent: []string{"hello hook"},
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
			name:        "stop names the task from the options",
			author:      "claude-code",
			payload:     `{"hook_event_name":"Stop"}`,
			opts:        func(key string) sdk.HookOpts { return sdk.HookOpts{Task: key} },
			wantEvent:   "session.stop",
			wantTask:    true,
			wantActions: []string{"queue.send"},
			wantMessage: "Session ended for task {key}",
		},
		{
			name:        "claude task completed finishes the task",
			author:      "claude",
			payload:     `{"hook_event_name":"TaskCompleted"}`,
			opts:        func(key string) sdk.HookOpts { return sdk.HookOpts{Task: key} },
			wantEvent:   "task.completed",
			wantTask:    true,
			wantActions: []string{"task.finish", "queue.send"},
			wantStatus:  "done",
			wantMessage: "Completed for task {key}",
		},
		{
			name:      "task completed without a task does nothing",
			author:    "claude",
			payload:   `{"hook_event_name":"TaskCompleted"}`,
			wantEvent: "task.completed",
		},
		{
			name:        "task completed for an unknown task still reports",
			author:      "my-agent",
			payload:     `{"event":"task.completed","task_id":"nosuchtask","content":"all green"}`,
			wantEvent:   "task.completed",
			wantActions: []string{"queue.send"},
			wantMessage: "all green",
		},
		{
			name:        "generic task failed blocks the task",
			author:      "my-agent",
			payload:     `{"event":"task.failed","task_id":"{key}","content":"tests broke"}`,
			wantEvent:   "task.failed",
			wantTask:    true,
			wantActions: []string{"task.move", "queue.send"},
			wantStatus:  "blocked",
			wantMessage: "tests broke",
		},
		{
			name:        "task failed for an unknown task still reports",
			author:      "my-agent",
			payload:     `{"event":"task.failed","task_id":"nosuchtask"}`,
			wantEvent:   "task.failed",
			wantActions: []string{"queue.send"},
			wantMessage: "Failed for task nosuchtask",
		},
		{
			name:        "task failed without content",
			author:      "my-agent",
			payload:     `{"event":"task.failed","task_id":"{key}"}`,
			wantEvent:   "task.failed",
			wantTask:    true,
			wantActions: []string{"task.move", "queue.send"},
			wantStatus:  "blocked",
			wantMessage: "Failed for task {key}",
		},
		{
			name:        "generic tool use records an audit",
			author:      "my-agent",
			payload:     `{"event":"tool.post","task_id":"{key}","content":"ran the tests"}`,
			wantEvent:   "tool.post",
			wantTask:    true,
			wantActions: []string{"audit.add"},
			wantAudit:   "ran the tests",
		},
		{
			name:      "tool use without content records nothing",
			author:    "my-agent",
			payload:   `{"event":"tool.post","task_id":"{key}"}`,
			wantEvent: "tool.post",
			wantTask:  true,
		},
		{
			name:      "gemini after tool without a task does nothing",
			author:    "gemini",
			payload:   `{"event":"AfterTool"}`,
			wantEvent: "tool.post",
		},
		{
			name:        "gemini session start",
			author:      "gemini",
			payload:     `{"event":"SessionStart"}`,
			wantEvent:   "session.start",
			wantActions: []string{"queue.ls"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := TestSetup(t, TestIsolated).Context(t.Context(), tt.author)
			task := addTask(t, c, "Hook task", spec, sdk.TaskAddOpts{Author: "alice"})
			if tt.seed != nil {
				tt.seed(t, c, task.Key)
			}
			expand := func(s string) string { return strings.ReplaceAll(s, "{key}", task.Key) }
			var payload []byte
			if tt.payload != "" {
				payload = []byte(expand(tt.payload))
			}
			var opts sdk.HookOpts
			if tt.opts != nil {
				opts = tt.opts(task.Key)
			}

			result, err := c.Hooks.Handle(tt.author, payload, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Handle(%q, %s) error = %v, want %v", tt.author, payload, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if result.Event != tt.wantEvent {
				t.Errorf("Handle(%q, %s).Event = %q, want %q", tt.author, payload, result.Event, tt.wantEvent)
			}
			if tt.wantTask && result.Task != task.Key {
				t.Errorf("Handle(%q, %s).Task = %q, want %q", tt.author, payload, result.Task, task.Key)
			}
			if !slices.Equal(result.Actions, tt.wantActions) {
				t.Errorf("Handle(%q, %s).Actions = %v, want %v", tt.author, payload, result.Actions, tt.wantActions)
			}
			for _, want := range tt.wantContent {
				if !strings.Contains(result.Content, want) {
					t.Errorf("Handle(%q, %s).Content = %q, want to contain %q", tt.author, payload, result.Content, want)
				}
			}
			if strings.Contains(result.Content, "not for you") {
				t.Errorf("Handle(%q, %s).Content = %q, lists a message directed elsewhere", tt.author, payload, result.Content)
			}
			if tt.wantStatus != "" {
				if st := readTask(t, c, task.Key).Status; st != tt.wantStatus {
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
					if m.Payload == expand(tt.wantMessage) && m.Type == "hook" && m.Author == tt.author && m.Source == "hook" {
						found = true
					}
				}
				if !found {
					t.Errorf("queue has no hook message %q by %s after hook; messages: %+v", expand(tt.wantMessage), tt.author, msgs)
				}
			}
			audits, err := c.Audits.List(sdk.AuditListOpts{Target: task.Key})
			if err != nil {
				t.Fatalf("Audits.List: %v", err)
			}
			switch {
			case tt.wantAudit != "" && (len(audits) != 1 || audits[0].Content != tt.wantAudit || audits[0].Author != tt.author):
				t.Errorf("audits after hook = %+v, want one by %s with %q", audits, tt.author, tt.wantAudit)
			case tt.wantAudit == "" && len(audits) != 0:
				t.Errorf("audits after hook = %+v, want none", audits)
			}
		})
	}
}
