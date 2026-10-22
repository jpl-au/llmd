package history_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/pkg/model/document"
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

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

func testRevertOpts() history.RevertOptions {
	return history.RevertOptions{Author: "test", Source: "cli"}
}

// mustWrite writes a document with the default test options and fails
// the test on error. Table setups use it to build version history.
func mustWrite(t *testing.T, s *llmd.Store, path, content string) *document.Document {
	t.Helper()
	doc, err := s.Documents.Write(t.Context(), path, content, testWriteOpts())
	if err != nil {
		t.Fatalf("Write(%q) error = %v", path, err)
	}
	return doc
}
