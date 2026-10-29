package bulk_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/bulk"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testImportOpts() bulk.ImportOptions {
	return bulk.ImportOptions{Author: "test", Source: "cli"}
}
