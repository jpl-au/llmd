package history_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/pkg/model/document"
)

func TestDiff(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "first version", opts)
	s.Documents.Write(ctx, "docs/readme", "second version", opts)

	v1, v2 := 1, 2
	doc1, _ := s.Documents.Read(ctx, "docs/readme", documents.ReadOptions{Version: &v1})
	doc2, _ := s.Documents.Read(ctx, "docs/readme", documents.ReadOptions{Version: &v2})

	result := history.Diff(doc1, doc2)

	if result.Doc1.Content != "first version" {
		t.Errorf("Doc1.Content = %q, want %q", result.Doc1.Content, "first version")
	}
	if result.Doc2.Content != "second version" {
		t.Errorf("Doc2.Content = %q, want %q", result.Doc2.Content, "second version")
	}
}

func TestDiff_WithLatest(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "first version", opts)
	s.Documents.Write(ctx, "docs/readme", "latest version", opts)

	v1 := 1
	doc1, _ := s.Documents.Read(ctx, "docs/readme", documents.ReadOptions{Version: &v1})
	doc2, _ := s.Documents.Read(ctx, "docs/readme")

	result := history.Diff(doc1, doc2)

	if result.Doc1.Content != "first version" {
		t.Errorf("Doc1.Content = %q, want %q", result.Doc1.Content, "first version")
	}
	if result.Doc2.Content != "latest version" {
		t.Errorf("Doc2.Content = %q, want %q", result.Doc2.Content, "latest version")
	}
}

func TestDiff_NilDoc(t *testing.T) {
	doc := &document.Document{Content: "hello"}
	result := history.Diff(doc, nil)

	if result.Doc1.Content != "hello" {
		t.Errorf("Doc1.Content = %q, want %q", result.Doc1.Content, "hello")
	}
	if result.Doc2 != nil {
		t.Errorf("Doc2 = %v, want nil", result.Doc2)
	}
}
