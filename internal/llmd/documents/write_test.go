package documents_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func TestWrite(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	doc, err := s.Documents.Write(ctx, "docs/readme", "# Hello", testWriteOpts())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if doc.Path != "docs/readme" {
		t.Errorf("Path = %q, want %q", doc.Path, "docs/readme")
	}
	if doc.Content != "# Hello" {
		t.Errorf("Content = %q, want %q", doc.Content, "# Hello")
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1", doc.Version)
	}
	if doc.Author != "test" {
		t.Errorf("Author = %q, want %q", doc.Author, "test")
	}
	if len(doc.Key) != 9 {
		t.Errorf("Key length = %d, want 9", len(doc.Key))
	}
	if doc.Meta == nil {
		t.Error("Meta is nil")
	} else {
		if doc.Meta.Size != 7 {
			t.Errorf("Meta.Size = %d, want 7", doc.Meta.Size)
		}
		if doc.Meta.Lines != 1 {
			t.Errorf("Meta.Lines = %d, want 1", doc.Meta.Lines)
		}
	}
}

func TestWrite_Versioning(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	doc1, _ := s.Documents.Write(ctx, "docs/readme", "v1", opts)
	doc2, _ := s.Documents.Write(ctx, "docs/readme", "v2", opts)
	doc3, _ := s.Documents.Write(ctx, "docs/readme", "v3", opts)

	if doc1.Version != 1 {
		t.Errorf("doc1.Version = %d, want 1", doc1.Version)
	}
	if doc2.Version != 2 {
		t.Errorf("doc2.Version = %d, want 2", doc2.Version)
	}
	if doc3.Version != 3 {
		t.Errorf("doc3.Version = %d, want 3", doc3.Version)
	}

	if doc1.Key != doc2.Key || doc2.Key != doc3.Key {
		t.Errorf("key must be stable across versions: v1=%q v2=%q v3=%q", doc1.Key, doc2.Key, doc3.Key)
	}
}

func TestWrite_SkipsUnchanged(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	doc1, _ := s.Documents.Write(ctx, "docs/readme", "same content", opts)
	doc2, _ := s.Documents.Write(ctx, "docs/readme", "same content", opts)

	if doc2.Version != doc1.Version {
		t.Errorf("unchanged content should not create new version: got %d, want %d", doc2.Version, doc1.Version)
	}
	if doc2.Key != doc1.Key {
		t.Errorf("unchanged content should return same key")
	}
}

func TestWrite_RequiresAuthor(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Write(ctx, "docs/readme", "content", documents.WriteOptions{
		Origin: core.Origin{Source: "cli"},
	})
	if err == nil {
		t.Error("Write() without author should fail")
	}
}

func TestWrite_RequiresSource(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Write(ctx, "docs/readme", "content", documents.WriteOptions{
		Origin: core.Origin{Author: "test"},
	})
	if err == nil {
		t.Error("Write() without source should fail")
	}
}
