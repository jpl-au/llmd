package history_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/history"
	"github.com/jpl-au/llmd/pkg/model/document"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

func testRevertOpts() history.RevertOptions {
	return history.RevertOptions{Author: "test", Source: "cli"}
}

// mustWrite writes a document with the default test options and fails
// the test on error. Table setups use it to build version history.
func mustWrite(t *testing.T, s *engine.Store, path, content string) *document.Document {
	t.Helper()
	doc, err := s.Documents.Write(t.Context(), path, content, testWriteOpts())
	if err != nil {
		t.Fatalf("Write(%q) error = %v", path, err)
	}
	return doc
}
