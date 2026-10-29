package search_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}
