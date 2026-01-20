package documents_test

import (
	"context"
	"testing"
)

func TestRestore(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Documents.Delete(ctx, "docs/readme", testDeleteOpts())

	err := s.Documents.Restore(ctx, "docs/readme", testRestoreOpts())
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	list, _ := s.Documents.List(ctx)
	if len(list) != 1 {
		t.Errorf("List() after restore returned %d items, want 1", len(list))
	}

	doc, err := s.Documents.Read(ctx, "docs/readme")
	if err != nil {
		t.Errorf("Read() after restore error = %v", err)
	}
	if doc.Content != "content" {
		t.Errorf("Content = %q, want %q", doc.Content, "content")
	}
}
