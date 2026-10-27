package documents_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/key"
)

// TestWriteTakenKey covers a key clash between processes: a new
// document whose key another path already holds takes a fresh one,
// while a new version of a path keeps that path's key.
func TestWriteTakenKey(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	t.Cleanup(func() { key.Force() })

	key.Force("zzzzzzzzz1")
	a, err := s.Documents.Write(ctx, "docs/a", "# A\n", testWriteOpts())
	if err != nil {
		t.Fatal(err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	b, err := s.Documents.Write(ctx, "docs/b", "# B\n", testWriteOpts())
	if err != nil {
		t.Fatalf("Write(docs/b) with a taken key: %v", err)
	}
	if a.Key != "zzzzzzzzz1" || b.Key != "zzzzzzzzz2" {
		t.Errorf("keys = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", a.Key, b.Key)
	}
	a2, err := s.Documents.Write(ctx, "docs/a", "# A again\n", testWriteOpts())
	if err != nil {
		t.Fatal(err)
	}
	if a2.Key != a.Key || a2.Version != 2 {
		t.Errorf("second version = key %q v%d, want key %q v2", a2.Key, a2.Version, a.Key)
	}
}
