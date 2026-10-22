package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestTaskChain covers the dependency chain rendering: a lone task,
// a two- and three-task chain (deepest dependency first, each level
// indented), the done tick, and the errors.
func TestTaskChain(t *testing.T) {
	tests := []struct {
		name      string
		noAuthor  bool
		depth     int  // tasks in the chain: T1 <- T2 <- T3
		baseDone  bool // move T1 to done first
		args      []string
		wantLines []string // expected text lines, with {t1}.. expanded
		wantKeys  []string // Data keys in order
		wantErr   error
	}{
		{
			name:      "single task",
			depth:     1,
			args:      []string{"chain", "{t1}"},
			wantLines: []string{"{t1}  T1  [backlog]"},
			wantKeys:  []string{"{t1}"},
		},
		{
			name:      "two tasks deepest first",
			depth:     2,
			args:      []string{"chain", "{t2}"},
			wantLines: []string{"{t1}  T1  [backlog]", "   └─ {t2}  T2  [backlog]"},
			wantKeys:  []string{"{t1}", "{t2}"},
		},
		{
			name:      "three task chain",
			depth:     3,
			args:      []string{"chain", "{t3}"},
			wantLines: []string{"{t1}  T1  [backlog]", "   └─ {t2}  T2  [backlog]", "      └─ {t3}  T3  [backlog]"},
			wantKeys:  []string{"{t1}", "{t2}", "{t3}"},
		},
		{
			name:      "done dependency is ticked",
			depth:     2,
			baseDone:  true,
			args:      []string{"chain", "{t2}"},
			wantLines: []string{"{t1}  T1  [done] ✓", "   └─ {t2}  T2  [backlog]"},
			wantKeys:  []string{"{t1}", "{t2}"},
		},
		{
			name:      "chain from the middle stops there",
			depth:     3,
			args:      []string{"chain", "{t2}"},
			wantLines: []string{"{t1}  T1  [backlog]", "   └─ {t2}  T2  [backlog]"},
			wantKeys:  []string{"{t1}", "{t2}"},
		},
		{name: "reads without author", noAuthor: true, depth: 1, args: []string{"chain", "{t1}"}, wantKeys: []string{"{t1}"}},
		{name: "missing task", depth: 1, args: []string{"chain", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", depth: 1, args: []string{"chain"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			vars := taskDepsSeed(t, c, tt.depth, tt.baseDone)
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, vars)

			if tt.wantErr != nil {
				taskWantErr(t, "task", args, runErr(t, c, "task", nil, args...), tt.wantErr, "")
				return
			}
			resp := run(t, c, "task", nil, args...)
			if tt.wantLines != nil {
				got := strings.Split(strings.TrimRight(text(resp), "\n"), "\n")
				if want := taskExpand(tt.wantLines, vars); !slices.Equal(got, want) {
					t.Errorf("task %v = %q, want lines %q", args, got, want)
				}
			}
			chain, ok := data(resp).([]*sdk.Task)
			if !ok {
				t.Fatalf("task %v Data = %T, want []*sdk.Task", args, data(resp))
			}
			keys := make([]string, len(chain))
			for i, tk := range chain {
				keys[i] = tk.Key
			}
			if want := taskExpand(tt.wantKeys, vars); !slices.Equal(keys, want) {
				t.Errorf("task %v Data keys = %v, want %v", args, keys, want)
			}
		})
	}
}

// TestTaskReady covers readiness: no dependency, a blocked and a
// satisfied direct dependency, a transitive block, and the errors.
func TestTaskReady(t *testing.T) {
	tests := []struct {
		name      string
		noAuthor  bool
		depth     int
		baseDone  bool
		args      []string
		wantReady bool
		wantErr   error
	}{
		{name: "no dependency is ready", depth: 1, args: []string{"ready", "{t1}"}, wantReady: true},
		{name: "unfinished dependency blocks", depth: 2, args: []string{"ready", "{t2}"}},
		{name: "done dependency is satisfied", depth: 2, baseDone: true, args: []string{"ready", "{t2}"}, wantReady: true},
		{name: "transitive unfinished dependency blocks", depth: 3, baseDone: true, args: []string{"ready", "{t3}"}},
		{name: "reads without author", noAuthor: true, depth: 1, args: []string{"ready", "{t1}"}, wantReady: true},
		{name: "missing task", depth: 1, args: []string{"ready", "nope"}, wantErr: sdk.ErrNotFound},
		{name: "no id", depth: 1, args: []string{"ready"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			vars := taskDepsSeed(t, c, tt.depth, tt.baseDone)
			if tt.noAuthor {
				c.Author = ""
			}
			args := taskExpand(tt.args, vars)

			if tt.wantErr != nil {
				taskWantErr(t, "task", args, runErr(t, c, "task", nil, args...), tt.wantErr, "")
				return
			}
			resp := run(t, c, "task", nil, args...)
			want := "Task " + args[1] + " is blocked by unfinished dependencies"
			if tt.wantReady {
				want = "Task " + args[1] + " is ready"
			}
			if got := text(resp); got != want {
				t.Errorf("task %v = %q, want %q", args, got, want)
			}
			d, ok := data(resp).(map[string]any)
			if !ok || d["key"] != args[1] || d["ready"] != tt.wantReady {
				t.Errorf("task %v Data = %#v, want key %s ready %v", args, data(resp), args[1], tt.wantReady)
			}
		})
	}
}

// taskDepsSeed creates depth tasks T1..Tn where each depends on the
// previous (T1 <- T2 <- T3). When baseDone is set, T1 is given a spec
// and moved to done. Returns {t1}..{tn} placeholders.
func taskDepsSeed(t *testing.T, c sdk.Context, depth int, baseDone bool) taskVars {
	t.Helper()
	vars := taskVars{}
	prev := ""
	for i := 1; i <= depth; i++ {
		title := "T" + string(rune('0'+i))
		var args []string
		if prev != "" {
			args = []string{"--depends-on", prev}
		}
		var body []byte
		if i == 1 && baseDone {
			body = taskBody(title)
		}
		tk := taskSeed(t, c, title, body, args...)
		if tk.DependsOn != prev {
			t.Fatalf("task %s DependsOn = %q, want %q", title, tk.DependsOn, prev)
		}
		vars[strings.ToLower(title)] = tk.Key
		prev = tk.Key
	}
	if baseDone {
		run(t, c, "task", nil, "move", vars["t1"], "done")
	}
	return vars
}

// TestTaskDeps ports test/smoke_deps.sh end to end on one store: add
// with --depends-on, show displays the dependency, chain, ready blocked
// then satisfied once the base task's spec is written and it is moved
// to done, set --depends-on, a three-task chain, cycle rejection as
// ErrCycle, and clearing a dependency.
func TestTaskDeps(t *testing.T) {
	c := newCLI(t, "smoke")

	t1 := taskSeed(t, c, "Base task", nil)
	t2 := taskSeed(t, c, "Dependent task", nil, "--depends-on", t1.Key)
	if t2.DependsOn != t1.Key {
		t.Fatalf("add --depends-on: DependsOn = %q, want %q", t2.DependsOn, t1.Key)
	}

	// show displays the dependency.
	if got := text(run(t, c, "task", nil, "show", t2.Key)); !strings.Contains(got, "| Depends On | "+t1.Key+" |") {
		t.Errorf("task show %s = %q, want Depends On %s", t2.Key, got, t1.Key)
	}

	// chain shows both tasks.
	if got := text(run(t, c, "task", nil, "chain", t2.Key)); !strings.Contains(got, t1.Key) || !strings.Contains(got, t2.Key) {
		t.Errorf("task chain %s = %q, want both %s and %s", t2.Key, got, t1.Key, t2.Key)
	}

	// ready reports blocked while the base task is open.
	if got := text(run(t, c, "task", nil, "ready", t2.Key)); !strings.Contains(got, "blocked") {
		t.Errorf("task ready %s = %q, want blocked", t2.Key, got)
	}

	// Writing the spec afterwards lets the base task move to done.
	writeDoc(t, c, "tasks/base-task", "# Base task\n\nSpec content.")
	run(t, c, "task", nil, "move", t1.Key, "done")
	if got := text(run(t, c, "task", nil, "ready", t2.Key)); got != "Task "+t2.Key+" is ready" {
		t.Errorf("task ready %s = %q, want ready", t2.Key, got)
	}

	// set --depends-on.
	t3 := taskSeed(t, c, "Third task", nil)
	run(t, c, "task", nil, "set", t3.Key, "--depends-on", t2.Key)
	if got := text(run(t, c, "task", nil, "show", t3.Key)); !strings.Contains(got, "| Depends On | "+t2.Key+" |") {
		t.Errorf("task show %s = %q, want Depends On %s", t3.Key, got, t2.Key)
	}

	// Three-task chain, deepest first.
	resp := run(t, c, "task", nil, "chain", t3.Key)
	got := text(resp)
	for _, k := range []string{t1.Key, t2.Key, t3.Key} {
		if !strings.Contains(got, k) {
			t.Errorf("task chain %s = %q, want %s", t3.Key, got, k)
		}
	}
	if chain, ok := data(resp).([]*sdk.Task); !ok || len(chain) != 3 || chain[0].Key != t1.Key || chain[2].Key != t3.Key {
		t.Errorf("task chain %s Data = %+v, want [%s %s %s]", t3.Key, data(resp), t1.Key, t2.Key, t3.Key)
	}

	// Cycle detection: T1 -> T3 would close the loop.
	if err := runErr(t, c, "task", nil, "set", t1.Key, "--depends-on", t3.Key); !errors.Is(err, sdk.ErrCycle) {
		t.Errorf("task set %s --depends-on %s error = %v, want %v", t1.Key, t3.Key, err, sdk.ErrCycle)
	}
	if got := taskRead(t, c, t1.Key).DependsOn; got != "" {
		t.Errorf("T1 DependsOn after rejected cycle = %q, want empty", got)
	}

	// Clearing a dependency.
	run(t, c, "task", nil, "set", t2.Key, "--depends-on", "")
	if got := text(run(t, c, "task", nil, "show", t2.Key)); strings.Contains(got, "Depends On") {
		t.Errorf("task show %s after clearing = %q, must not show Depends On", t2.Key, got)
	}
	if got := taskRead(t, c, t2.Key).DependsOn; got != "" {
		t.Errorf("T2 DependsOn after clearing = %q, want empty", got)
	}
}
