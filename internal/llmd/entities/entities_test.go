package entities_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	s, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testWriteOpts() entities.WriteOptions {
	return entities.WriteOptions{
		Origin: core.Origin{Author: "test", Source: "cli"},
	}
}

func testDeleteOpts() entities.DeleteOptions {
	return entities.DeleteOptions{
		Origin: core.Origin{Author: "test", Source: "cli"},
	}
}
