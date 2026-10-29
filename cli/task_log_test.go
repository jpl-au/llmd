package cli

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// taskLogClock installs a strictly increasing clock so every audit
// event gets a distinct timestamp: the log is ordered by timestamp
// alone, and same-millisecond events would otherwise come back in an
// arbitrary order.
func taskLogClock(t *testing.T) {
	t.Helper()
	clocktest.Sequence(t, 1_700_000_000_000, 1)
}

// taskLogActions returns the Action of each event in order.
func taskLogActions(events []sdk.TaskEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Action
	}
	return out
}

// TestTaskLog covers the audit history: every action name a task
// mutation records, newest-first ordering, -n, the all-tasks view
// with its TASK column, history of a deleted task, and the errors.
// Each case runs against a store where Alpha has been through the
// full lifecycle (created ... restored) and Beta was only created.
func TestTaskLog(t *testing.T) {
	// Alpha's history, newest first.
	alphaActions := []string{
		"restored", "deleted", "unflagged", "flagged", "edited:depends_on",
		"edited:position", "edited:branch", "edited:assigned_to",
		"edited:priority", "edited:title", "moved", "created",
	}

	tests := []struct {
		name        string
		noAuthor    bool
		args        []string // {alpha}, {beta}, {dep} are expanded
		wantText    []string
		wantAbsent  []string
		wantActions []string // nil skips the check
		wantErr     error
	}{
		{
			name:        "full history newest first",
			args:        []string{"log", "{alpha}"},
			wantText:    []string{"TIME", "ACTOR", "ACTION", "OLD", "NEW", "alice", "created", "moved", "in-progress", "restored"},
			wantAbsent:  []string{"TASK"},
			wantActions: alphaActions,
		},
		{
			name:        "-n limits to the most recent events",
			args:        []string{"log", "{alpha}", "-n", "2"},
			wantText:    []string{"restored", "deleted"},
			wantAbsent:  []string{"created"},
			wantActions: alphaActions[:2],
		},
		{
			name:        "-n attached form",
			args:        []string{"log", "-n3", "{alpha}"},
			wantActions: alphaActions[:3],
		},
		{
			// Newest first across tasks: Beta's creation, Alpha's lifecycle,
			// then the creation of Dep and of Alpha itself.
			name:        "all history adds the TASK column",
			args:        []string{"log"},
			wantText:    []string{"TASK", "{alpha}", "{beta}"},
			wantActions: append(append([]string{"created"}, alphaActions[:11]...), "created", "created"),
		},
		{
			name:        "history of another task is separate",
			args:        []string{"log", "{beta}"},
			wantAbsent:  []string{"moved", "{alpha}"},
			wantActions: []string{"created"},
		},
		{name: "reads without author", noAuthor: true, args: []string{"log", "{alpha}"}, wantActions: alphaActions},
		{name: "missing task", args: []string{"log", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "unknown flag", args: []string{"log", "{alpha}", "--bogus"}, wantErr: sdk.ErrInvalidArg},
		{name: "-n not an integer", args: []string{"log", "{alpha}", "-n", "many"}, wantErr: sdk.ErrInvalidArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskLogClock(t)
			c := newCLI(t, "alice")
			alpha := taskSeed(t, c, "Alpha", taskBody("Alpha"))
			dep := taskSeed(t, c, "Dep", nil)
			run(t, c, "task", nil, "move", alpha.Key, "in-progress")
			run(t, c, "task", nil, "set", alpha.Key, "--title", "Alpha 2")
			run(t, c, "task", nil, "set", alpha.Key, "--priority", "2")
			run(t, c, "task", nil, "set", alpha.Key, "--assign", "bob")
			run(t, c, "task", nil, "set", alpha.Key, "--branch", "feature-x")
			run(t, c, "task", nil, "set", alpha.Key, "--position", "0")
			run(t, c, "task", nil, "set", alpha.Key, "--depends-on", dep.Key)
			run(t, c, "task", nil, "set", alpha.Key, "--flag", "blocked")
			run(t, c, "task", nil, "set", alpha.Key, "--unflag", "blocked")
			run(t, c, "task", nil, "rm", alpha.Key)
			run(t, c, "task", nil, "restore", alpha.Key)
			beta := taskSeed(t, c, "Beta", nil)
			vars := taskVars{"alpha": alpha.Key, "beta": beta.Key, "dep": dep.Key}
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, vars)

			if tt.wantErr != nil {
				taskWantErr(t, "task", args, runErr(t, c, "task", nil, args...), tt.wantErr, "")
				return
			}
			resp := run(t, c, "task", nil, args...)
			taskWantText(t, "task", args, text(resp), taskExpand(tt.wantText, vars), taskExpand(tt.wantAbsent, vars))
			events, ok := data(resp).([]sdk.TaskEvent)
			if !ok {
				t.Fatalf("task %v Data = %T, want []sdk.TaskEvent", args, data(resp))
			}
			if tt.wantActions != nil {
				if got := taskLogActions(events); !slices.Equal(got, tt.wantActions) {
					t.Errorf("task %v actions = %v, want %v", args, got, tt.wantActions)
				}
			}
			for _, e := range events {
				if e.Actor != "alice" {
					t.Errorf("event %+v Actor = %q, want alice", e, e.Actor)
				}
				if e.Action == "moved" && (e.Subject != alpha.Key || e.OldValue != "backlog" || e.NewValue != "in-progress") {
					t.Errorf("moved event = %+v, want %s backlog -> in-progress", e, alpha.Key)
				}
				if e.Action == "edited:title" && (e.OldValue != "Alpha" || e.NewValue != "Alpha 2") {
					t.Errorf("edited:title event = %+v, want Alpha -> Alpha 2", e)
				}
			}
		})
	}
}

// TestTaskLogEmpty covers the two empty states: a store with no task
// history at all, and a task that was deleted (its history is still
// readable).
func TestTaskLogEmpty(t *testing.T) {
	t.Run("no history", func(t *testing.T) {
		c := newCLI(t, "alice")
		resp := run(t, c, "task", nil, "log")
		taskWantText(t, "task", []string{"log"}, text(resp), []string{"no history"}, []string{"TIME"})
		if events, ok := data(resp).([]sdk.TaskEvent); !ok || len(events) != 0 {
			t.Errorf("task log Data = %#v, want an empty []sdk.TaskEvent", data(resp))
		}
	})

	t.Run("deleted task keeps its history", func(t *testing.T) {
		taskLogClock(t)
		c := newCLI(t, "alice")
		alpha := taskSeed(t, c, "Alpha", nil)
		run(t, c, "task", nil, "rm", alpha.Key)
		args := []string{"log", alpha.Key}
		resp := run(t, c, "task", nil, args...)
		events, ok := data(resp).([]sdk.TaskEvent)
		if !ok {
			t.Fatalf("task %v Data = %T, want []sdk.TaskEvent", args, data(resp))
		}
		if got := taskLogActions(events); !slices.Equal(got, []string{"deleted", "created"}) {
			t.Errorf("task %v actions = %v, want [deleted created]", args, got)
		}
		if events[0].OldValue != "backlog" {
			t.Errorf("deleted event OldValue = %q, want the column it left", events[0].OldValue)
		}
	})
}
