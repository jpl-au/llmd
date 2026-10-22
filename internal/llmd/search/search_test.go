package search_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/llmdtest"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	return llmdtest.Open(t)
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}
