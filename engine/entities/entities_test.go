package entities_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/entities"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
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
func writeEntity(t *testing.T, s *engine.Store, namespace, value, relation string) string {
	t.Helper()
	opts := testWriteOpts()
	opts.Relation = relation
	ent, err := s.Entities.Write(t.Context(), namespace, value, opts)
	if err != nil {
		t.Fatalf("Write(%s, %s) error = %v", namespace, value, err)
	}
	return ent.Key
}
