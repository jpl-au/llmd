package documents_test

import (
	"context"
	"testing"
)

func TestExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Document doesn't exist
	if ok, err := s.Documents.Exists(ctx, "docs/readme"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true, want false for non-existent document")
	}

	// Create document
	doc, err := s.Documents.Write(ctx, "docs/readme", "# Hello", testWriteOpts())
	if err != nil {
		t.Fatal(err)
	}

	// Document exists by path
	if ok, err := s.Documents.Exists(ctx, "docs/readme"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true for existing document")
	}

	// Document exists by key
	if ok, err := s.Documents.Exists(ctx, doc.Key); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true for existing document by key")
	}
}

func TestExists_Deleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create and delete document
	_, err := s.Documents.Write(ctx, "docs/readme", "# Hello", testWriteOpts())
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Documents.Delete(ctx, "docs/readme", testDeleteOpts()); err != nil {
		t.Fatal(err)
	}

	// Deleted document should not exist
	if ok, err := s.Documents.Exists(ctx, "docs/readme"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true, want false for deleted document")
	}
}
