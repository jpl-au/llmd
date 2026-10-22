package bulk_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/llmdtest"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	return llmdtest.Open(t)
}

func testImportOpts() bulk.ImportOptions {
	return bulk.ImportOptions{Author: "test", Source: "cli"}
}
