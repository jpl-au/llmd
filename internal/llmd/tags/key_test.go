package tags_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/key"
)

// TestAddTakenKey covers a key clash between processes: the second tag
// is stored under a fresh key.
func TestAddTakenKey(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	t.Cleanup(func() { key.Force() })
	if _, err := s.Documents.Write(ctx, "docs/a", "# A\n", testWriteOpts()); err != nil {
		t.Fatal(err)
	}

	key.Force("zzzzzzzzz1")
	a, err := s.Tags.Add(ctx, "docs/a", "one", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	key.Force("zzzzzzzzz1", "zzzzzzzzz2")
	b, err := s.Tags.Add(ctx, "docs/a", "two", testOpts())
	if err != nil {
		t.Fatalf("Add(two) with a taken key: %v", err)
	}
	if a.Key != "zzzzzzzzz1" || b.Key != "zzzzzzzzz2" {
		t.Errorf("keys = %q, %q, want zzzzzzzzz1 and the fresh zzzzzzzzz2", a.Key, b.Key)
	}
}
