package history_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/history"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// TestRevertKeepsKey covers that a reverted version carries the
// document's key, like every other version of the path.
func TestRevertKeepsKey(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	origin := core.Origin{Author: "test", Source: "cli"}
	v1, err := s.Documents.Write(ctx, "docs/a", "one\n", documents.WriteOptions{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Documents.Write(ctx, "docs/a", "two\n", documents.WriteOptions{Origin: origin}); err != nil {
		t.Fatal(err)
	}
	v3, err := s.History.Revert(ctx, "docs/a", 1, history.RevertOptions{Origin: origin})
	if err != nil {
		t.Fatal(err)
	}
	if v3.Key != v1.Key || v3.Version != 3 {
		t.Errorf("reverted version = key %q v%d, want key %q v3", v3.Key, v3.Version, v1.Key)
	}
}
