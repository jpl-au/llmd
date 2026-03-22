package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/pkg/model/core"
)

var testOrigin = core.Origin{Author: "alice", Source: "test"}

func TestAddWithDependsOn(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, err := ts.Add(ctx, "Base task", nil, AddOptions{Origin: testOrigin})
	if err != nil {
		t.Fatal(err)
	}

	t2, err := ts.Add(ctx, "Dependent task", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: t1.Key,
	})
	if err != nil {
		t.Fatal(err)
	}
	if t2.DependsOn != t1.Key {
		t.Errorf("DependsOn = %q, want %q", t2.DependsOn, t1.Key)
	}

	// Read back and verify.
	got, err := ts.Read(ctx, t2.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.DependsOn != t1.Key {
		t.Errorf("DependsOn after read = %q, want %q", got.DependsOn, t1.Key)
	}
}

func TestAddDependsOnNotFound(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	_, err := ts.Add(ctx, "Bad dep", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent dependency target")
	}
}

func TestSetDependsOn(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "First", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Second", nil, AddOptions{Origin: testOrigin})

	dep := t1.Key
	if err := ts.Set(ctx, t2.Key, "alice", SetOptions{DependsOn: &dep}); err != nil {
		t.Fatal(err)
	}

	got, _ := ts.Read(ctx, t2.Key)
	if got.DependsOn != t1.Key {
		t.Errorf("DependsOn = %q, want %q", got.DependsOn, t1.Key)
	}
}

func TestClearDependsOn(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "First", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Second", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: t1.Key,
	})

	empty := ""
	if err := ts.Set(ctx, t2.Key, "alice", SetOptions{DependsOn: &empty}); err != nil {
		t.Fatal(err)
	}

	got, _ := ts.Read(ctx, t2.Key)
	if got.DependsOn != "" {
		t.Errorf("DependsOn = %q, want empty", got.DependsOn)
	}
}

func TestSetDependsOnCycleSelf(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Self ref", nil, AddOptions{Origin: testOrigin})

	dep := t1.Key
	err := ts.Set(ctx, t1.Key, "alice", SetOptions{DependsOn: &dep})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v, want ErrCycle", err)
	}
}

func TestSetDependsOnCycleChain(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "A", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "B", nil, AddOptions{Origin: testOrigin, DependsOn: t1.Key})
	t3, _ := ts.Add(ctx, "C", nil, AddOptions{Origin: testOrigin, DependsOn: t2.Key})

	// Try to make A depend on C (would create A -> C -> B -> A).
	dep := t3.Key
	err := ts.Set(ctx, t1.Key, "alice", SetOptions{DependsOn: &dep})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v, want ErrCycle", err)
	}
}

func TestDep(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Base", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Dependent", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: t1.Key,
	})

	dep, err := ts.Dep(ctx, t2.Key)
	if err != nil {
		t.Fatal(err)
	}
	if dep.Key != t1.Key {
		t.Errorf("dep.Key = %q, want %q", dep.Key, t1.Key)
	}

	// No dependency returns nil.
	dep, err = ts.Dep(ctx, t1.Key)
	if err != nil {
		t.Fatal(err)
	}
	if dep != nil {
		t.Errorf("expected nil, got %v", dep)
	}
}

func TestDependents(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Root", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Child A", nil, AddOptions{Origin: testOrigin, DependsOn: t1.Key})
	t3, _ := ts.Add(ctx, "Child B", nil, AddOptions{Origin: testOrigin, DependsOn: t1.Key})

	deps, err := ts.Dependents(ctx, t1.Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Fatalf("len = %d, want 2", len(deps))
	}

	keys := map[string]bool{deps[0].Key: true, deps[1].Key: true}
	if !keys[t2.Key] || !keys[t3.Key] {
		t.Errorf("dependents = %v, want %s and %s", keys, t2.Key, t3.Key)
	}
}

func TestChain(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Root", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Middle", nil, AddOptions{Origin: testOrigin, DependsOn: t1.Key})
	t3, _ := ts.Add(ctx, "Leaf", nil, AddOptions{Origin: testOrigin, DependsOn: t2.Key})

	chain, err := ts.Chain(ctx, t3.Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 {
		t.Fatalf("len = %d, want 3", len(chain))
	}

	// Deepest dependency first.
	if chain[0].Key != t1.Key {
		t.Errorf("chain[0] = %q, want %q", chain[0].Key, t1.Key)
	}
	if chain[1].Key != t2.Key {
		t.Errorf("chain[1] = %q, want %q", chain[1].Key, t2.Key)
	}
	if chain[2].Key != t3.Key {
		t.Errorf("chain[2] = %q, want %q", chain[2].Key, t3.Key)
	}
}

func TestChainNoDeps(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Solo", nil, AddOptions{Origin: testOrigin})

	chain, err := ts.Chain(ctx, t1.Key)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 1 {
		t.Fatalf("len = %d, want 1", len(chain))
	}
	if chain[0].Key != t1.Key {
		t.Errorf("chain[0] = %q, want %q", chain[0].Key, t1.Key)
	}
}

func TestReadyNoDeps(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "No deps", nil, AddOptions{Origin: testOrigin})

	ready, err := ts.Ready(ctx, t1.Key)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Error("expected ready = true for task with no dependencies")
	}
}

func TestReadyBlocked(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Incomplete", nil, AddOptions{Origin: testOrigin})
	t2, _ := ts.Add(ctx, "Waiting", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: t1.Key,
	})

	ready, err := ts.Ready(ctx, t2.Key)
	if err != nil {
		t.Fatal(err)
	}
	if ready {
		t.Error("expected ready = false when dependency is not done")
	}
}

func TestReadySatisfied(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

	t1, _ := ts.Add(ctx, "Done task", []byte("# Done task\n\nSpec."), AddOptions{Origin: testOrigin})

	// Move dependency to done.
	if err := ts.Move(ctx, t1.Key, "done", "alice"); err != nil {
		t.Fatal(err)
	}

	t2, _ := ts.Add(ctx, "Ready task", nil, AddOptions{
		Origin:    testOrigin,
		DependsOn: t1.Key,
	})

	ready, err := ts.Ready(ctx, t2.Key)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Error("expected ready = true when dependency is done")
	}
}

func TestDependsOnAuditLog(t *testing.T) {
	ts := setup(t)
	ctx := context.Background()

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
