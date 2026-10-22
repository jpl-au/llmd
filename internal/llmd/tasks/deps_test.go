package tasks

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/task"
)

var testOrigin = core.Origin{Author: "alice", Source: "test"}

// depKeys maps fixture labels to generated task keys.
type depKeys map[string]string

// key resolves a fixture label to its task key. Unknown labels are
// returned unchanged so a case can name a key that does not exist.
func (k depKeys) key(label string) string {
	if key, ok := k[label]; ok {
		return key
	}
	return label
}

// keys resolves several labels at once.
func (k depKeys) keys(labels []string) []string {
	var out []string
	for _, l := range labels {
		out = append(out, k.key(l))
	}
	return out
}

// depFixture builds a small dependency graph and returns its keys by
// label:
//
//	root <- a <- leaf
//	root <- b
//	solo
//
// Every task has a spec body so it can be moved out of the backlog.
func depFixture(t *testing.T, ts *Tasks) depKeys {
	t.Helper()
	ctx := t.Context()
	keys := depKeys{}
	for _, f := range []struct{ label, title, dependsOn string }{
		{"root", "Root", ""},
		{"a", "A", "root"},
		{"b", "B", "root"},
		{"leaf", "Leaf", "a"},
		{"solo", "Solo", ""},
	} {
		tsk, err := ts.Add(ctx, f.title, []byte("# "+f.title+"\n\nSpec."), AddOptions{
			Origin:    testOrigin,
			DependsOn: keys.key(f.dependsOn),
		})
		if err != nil {
			t.Fatalf("Add(%q) error = %v", f.title, err)
		}
		keys[f.label] = tsk.Key
	}
	return keys
}

// taskKeys extracts the keys from a task slice.
func taskKeys(tasks []*task.Task) []string {
	var out []string
	for _, tsk := range tasks {
		out = append(out, tsk.Key)
	}
	return out
}

// TestAddDependsOn covers dependency validation at creation time.
func TestAddDependsOn(t *testing.T) {
	tests := []struct {
		name      string
		dependsOn string // fixture label, or a literal key
		wantErr   error
	}{
		{"existing target", "root", nil},
		{"missing target", "nonexistent", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()
			keys := depFixture(t, ts)
			dep := keys.key(tt.dependsOn)

			tsk, err := ts.Add(ctx, "Dependent task", nil, AddOptions{Origin: testOrigin, DependsOn: dep})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(DependsOn=%q) error = %v, want %v", dep, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if tsk.DependsOn != dep {
				t.Errorf("Add(DependsOn=%q).DependsOn = %q, want %q", dep, tsk.DependsOn, dep)
			}

			// Read back and verify.
			got, err := ts.Read(ctx, tsk.Key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", tsk.Key, err)
			}
			if got.DependsOn != dep {
				t.Errorf("Read(%q).DependsOn = %q, want %q", tsk.Key, got.DependsOn, dep)
			}
		})
	}
}

// TestSetDependsOn covers setting, clearing and validating a task's
// dependency, including cycle detection. A rejected Set must leave
// the dependency unchanged.
func TestSetDependsOn(t *testing.T) {
	tests := []struct {
		name    string
		key     string // fixture label of the task to update
		dep     string // fixture label, literal key, or "" to clear
		wantErr error
	}{
		{"set", "solo", "root", nil},
		{"clear", "a", "", nil},
		{"self cycle", "root", "root", ErrCycle},
		// root <- a <- leaf, so root depending on leaf closes the loop.
		{"chain cycle", "root", "leaf", ErrCycle},
		{"missing target", "solo", "nonexistent", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()
			keys := depFixture(t, ts)
			key, dep := keys.key(tt.key), keys.key(tt.dep)

			before, err := ts.Read(ctx, key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", key, err)
			}

			err = ts.Set(ctx, key, "alice", SetOptions{DependsOn: &dep})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Set(%q, DependsOn=%q) error = %v, want %v", key, dep, err, tt.wantErr)
			}

			want := before.DependsOn
			if tt.wantErr == nil {
				want = dep
			}
			got, err := ts.Read(ctx, key)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", key, err)
			}
			if got.DependsOn != want {
				t.Errorf("after Set(%q, DependsOn=%q): DependsOn = %q, want %q", key, dep, got.DependsOn, want)
			}
		})
	}
}

// TestDep covers the single upstream dependency lookup. An empty want
// means Dep must return nil.
func TestDep(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"has dependency", "a", "root"},
		{"root has none", "root", ""},
		{"solo has none", "solo", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			keys := depFixture(t, ts)
			key := keys.key(tt.key)

			dep, err := ts.Dep(t.Context(), key)
			if err != nil {
				t.Fatalf("Dep(%q) error = %v", key, err)
			}
			if tt.want == "" {
				if dep != nil {
					t.Errorf("Dep(%q) = %v, want nil", key, dep)
				}
				return
			}
			if dep == nil || dep.Key != keys.key(tt.want) {
				t.Errorf("Dep(%q) = %v, want key %q", key, dep, keys.key(tt.want))
			}
		})
	}
}

// TestDependents covers the downstream lookup. Order is not defined,
// so keys are compared sorted.
func TestDependents(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want []string
	}{
		{"two children", "root", []string{"a", "b"}},
		{"one child", "a", []string{"leaf"}},
		{"none", "solo", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			keys := depFixture(t, ts)
			key := keys.key(tt.key)

			deps, err := ts.Dependents(t.Context(), key)
			if err != nil {
				t.Fatalf("Dependents(%q) error = %v", key, err)
			}
			got, want := taskKeys(deps), keys.keys(tt.want)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("Dependents(%q) = %v, want %v", key, got, want)
			}
		})
	}
}

// TestChain covers the full upstream walk, deepest dependency first
// and the queried task last.
func TestChain(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want []string
	}{
		{"three deep", "leaf", []string{"root", "a", "leaf"}},
		{"two deep", "b", []string{"root", "b"}},
		{"root only", "root", []string{"root"}},
		{"no deps", "solo", []string{"solo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			keys := depFixture(t, ts)
			key := keys.key(tt.key)

			chain, err := ts.Chain(t.Context(), key)
			if err != nil {
				t.Fatalf("Chain(%q) error = %v", key, err)
			}
			got, want := taskKeys(chain), keys.keys(tt.want)
			if !slices.Equal(got, want) {
				t.Errorf("Chain(%q) = %v, want %v", key, got, want)
			}
		})
	}
}

// TestReady covers readiness across the whole chain: every upstream
// task must be done. The done list names fixture tasks moved to
// "done" before the check.
func TestReady(t *testing.T) {
	tests := []struct {
		name string
		key  string
		done []string
		want bool
	}{
		{"no deps", "solo", nil, true},
		{"blocked", "a", nil, false},
		{"satisfied", "a", []string{"root"}, true},
		{"chain partially done", "leaf", []string{"root"}, false},
		{"chain fully done", "leaf", []string{"root", "a"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := setup(t)
			ctx := t.Context()
			keys := depFixture(t, ts)
			for _, label := range tt.done {
				if err := ts.Move(ctx, keys.key(label), "done", "alice"); err != nil {
					t.Fatalf("Move(%q, done) error = %v", label, err)
				}
			}
			key := keys.key(tt.key)

			ready, err := ts.Ready(ctx, key)
			if err != nil {
				t.Fatalf("Ready(%q) error = %v", key, err)
			}
			if ready != tt.want {
				t.Errorf("Ready(%q) = %v, want %v", key, ready, tt.want)
			}
		})
	}
}

func TestDependsOnAuditLog(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()

	t1, _ := ts.Add(ctx, "Dep target", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Has dep", nil, AddOptions{Origin: testOrigin})

	dep := t1.Key
	if err := ts.Set(ctx, t2.Key, "alice", SetOptions{DependsOn: &dep}); err != nil {
		t.Fatal(err)
	}

	events, err := ts.Log(ctx, t2.Key, 0)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range events {
		if e.Action == "edited:depends_on" {
			found = true
			if e.NewValue != t1.Key {
				t.Errorf("NewValue = %q, want %q", e.NewValue, t1.Key)
			}
		}
	}
	if !found {
		t.Error("missing edited:depends_on audit entry")
	}
}
