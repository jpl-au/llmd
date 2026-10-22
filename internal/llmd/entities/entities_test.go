package entities_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/entities"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	s, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	})
	return s
}

func testWriteOpts() entities.WriteOptions {
	return entities.WriteOptions{
		Author: "test", Source: "cli",
	}
}

func testDeleteOpts() entities.DeleteOptions {
	return entities.DeleteOptions{
		Author: "test", Source: "cli",
	}
}

// writeEntity writes an entity with an optional relation and returns its key.
func writeEntity(t *testing.T, s *llmd.Store, namespace, value, relation string) string {
	t.Helper()
	opts := testWriteOpts()
	opts.Relation = relation
	ent, err := s.Entities.Write(t.Context(), namespace, value, opts)
	if err != nil {
		t.Fatalf("Write(%s, %s) error = %v", namespace, value, err)
	}
	return ent.Key
}
