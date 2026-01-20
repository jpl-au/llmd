package links_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/core"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/links"
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

func testOpts() links.Options {
	return links.Options{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}
