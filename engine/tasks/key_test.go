package tasks

import (
	"testing"

	"github.com/jpl-au/llmd/engine/key"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// TestAddTakenKey covers a key clash between processes: the second task
// is stored under a fresh key and both can be read.
func TestAddTakenKey(t *testing.T) {
	ts := setup(t)
	ctx := t.Context()
	origin := core.Origin{Author: "alice", Source: "test"}
	t.Cleanup(func() { key.Force() })

	key.Force("zzzzzzzzz1")
	a, err := ts.Add(ctx, "First", nil, AddOptions{Origin: origin})
	if err != nil {
		t.Fatalf("Add(First): %v", err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	b, err := ts.Add(ctx, "Second", nil, AddOptions{Origin: origin})
	if err != nil {
		t.Fatalf("Add(Second) with a taken key: %v", err)
	}
	if a.Key != "zzzzzzzzz1" || b.Key != "zzzzzzzzz2" {
		t.Errorf("keys = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", a.Key, b.Key)
	}
	got, err := ts.Read(ctx, "zzzzzzzzz2")
	if err != nil || got.Title != "Second" {
		t.Errorf("Read(zzzzzzzzz2) = %+v, %v, want the second task", got, err)
	}
}
