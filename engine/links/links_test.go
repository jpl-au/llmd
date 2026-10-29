package links_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine/enginetest"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/links"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testOpts() links.Options {
	return links.Options{Author: "test", Source: "cli"}
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

// writeDocs writes one document per path and returns each path's key, so a
// case can address documents by key as well as by path.
func writeDocs(t *testing.T, s *engine.Store, paths ...string) map[string]string {
	t.Helper()
	keys := make(map[string]string, len(paths))
	for _, p := range paths {
		doc, err := s.Documents.Write(t.Context(), p, "content "+p, testWriteOpts())
		if err != nil {
			t.Fatalf("Write(%s) error = %v", p, err)
		}
		keys[p] = doc.Key
	}
	return keys
}

// addLink adds a from->to link with an optional label.
func addLink(t *testing.T, s *engine.Store, from, to, label string) {
	t.Helper()
	opts := testOpts()
	opts.Label = label
	if _, err := s.Links.Add(t.Context(), from, to, opts); err != nil {
		t.Fatalf("Add(%s, %s, label=%q) error = %v", from, to, label, err)
	}
}
