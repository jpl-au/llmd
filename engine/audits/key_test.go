package audits

import (
	"testing"

	"github.com/jpl-au/llmd/engine/key"
)

// TestAddTakenKey covers a key clash between processes: the second
// audit is stored under a fresh id.
func TestAddTakenKey(t *testing.T) {
	a := newTestStore(t)
	ctx := t.Context()
	t.Cleanup(func() { key.Force() })

	key.Force("zzzzzzzzz1")
	first, err := a.Add(ctx, AddOptions{Target: "docs/a", Content: "one", Author: "r"})
	if err != nil {
		t.Fatal(err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	second, err := a.Add(ctx, AddOptions{Target: "docs/a", Content: "two", Author: "r"})
	if err != nil {
		t.Fatalf("Add(two) with a taken id: %v", err)
	}
	if first.ID != "zzzzzzzzz1" || second.ID != "zzzzzzzzz2" {
		t.Errorf("ids = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", first.ID, second.ID)
	}
}
