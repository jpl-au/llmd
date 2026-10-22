package links_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/links"
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

func testOpts() links.Options {
	return links.Options{Author: "test", Source: "cli"}
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

// writeDocs writes one document per path and returns each path's key, so a
// case can address documents by key as well as by path.
func writeDocs(t *testing.T, s *llmd.Store, paths ...string) map[string]string {
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
func addLink(t *testing.T, s *llmd.Store, from, to, label string) {
	t.Helper()
	opts := testOpts()
	opts.Label = label
	if _, err := s.Links.Add(t.Context(), from, to, opts); err != nil {
		t.Fatalf("Add(%s, %s, label=%q) error = %v", from, to, label, err)
	}
}
