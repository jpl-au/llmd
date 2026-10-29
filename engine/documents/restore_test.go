package documents_test

import (
	"testing"
)

func TestRestore(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()

	mustWrite(t, s, "docs/readme", "content")
	mustDelete(t, s, "docs/readme")

	if err := s.Documents.Restore(ctx, "docs/readme", testRestoreOpts()); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	list, err := s.Documents.List(ctx)
	if err != nil {
		t.Fatalf("List() after Restore error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List() after Restore returned %d items, want 1", len(list))
	}

	doc, err := s.Documents.Read(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("Read() after Restore error = %v", err)
	}
	if doc.Content != "content" {
		t.Errorf("Read() after Restore: Content = %q, want %q", doc.Content, "content")
	}
}
