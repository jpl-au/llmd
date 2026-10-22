package tags_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/llmdtest"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	return llmdtest.Open(t)
}

func testOpts() tags.Options {
	return tags.Options{Author: "test", Source: "cli"}
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}
