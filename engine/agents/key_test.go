package agents

import (
	"testing"

	"github.com/jpl-au/llmd/engine/key"
)

// TestRecordTakenKey covers a key clash between processes: the second
// run is stored under a fresh key.
func TestRecordTakenKey(t *testing.T) {
	a := setup(t)
	ctx := t.Context()
	t.Cleanup(func() { key.Force() })

	key.Force("zzzzzzzzz1")
	first, err := a.Record(ctx, RecordOpts{TaskKey: "task1", Agent: "claude-code", PID: 1, Author: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	second, err := a.Record(ctx, RecordOpts{TaskKey: "task2", Agent: "claude-code", PID: 2, Author: "dev"})
	if err != nil {
		t.Fatalf("Record(task2) with a taken key: %v", err)
	}
	if first.Key != "zzzzzzzzz1" || second.Key != "zzzzzzzzz2" {
		t.Errorf("keys = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", first.Key, second.Key)
	}
}
