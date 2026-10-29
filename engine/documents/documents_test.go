package documents_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/pkg/model/document"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

func testDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{Author: "test", Source: "cli"}
}

func testRestoreOpts() documents.RestoreOptions {
	return documents.RestoreOptions{Author: "test", Source: "cli"}
}

func testEditOpts() documents.EditOptions {
	return documents.EditOptions{Author: "test", Source: "cli"}
}

func testMoveOpts() documents.MoveOptions {
	return documents.MoveOptions{Author: "test", Source: "cli"}
}

func testCopyOpts() documents.CopyOptions {
	return documents.CopyOptions{Author: "test", Source: "cli"}
}

// mustWrite writes a document with the default test options and fails
// the test on error. Table setups use it to build precondition state.
func mustWrite(t *testing.T, s *engine.Store, path, content string) *document.Document {
	t.Helper()
	doc, err := s.Documents.Write(t.Context(), path, content, testWriteOpts())
	if err != nil {
		t.Fatalf("Write(%q) error = %v", path, err)
	}
	return doc
}

// mustDelete soft-deletes a document with the default test options and
// fails the test on error.
func mustDelete(t *testing.T, s *engine.Store, path string) {
	t.Helper()
	if err := s.Documents.Delete(t.Context(), path, testDeleteOpts()); err != nil {
		t.Fatalf("Delete(%q) error = %v", path, err)
	}
}
