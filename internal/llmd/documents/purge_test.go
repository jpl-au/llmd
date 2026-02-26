package documents_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestPurge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create and delete some documents
	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Documents.Write(ctx, "docs/c", "content c", testWriteOpts())

	if err := s.Documents.Delete(ctx, "docs/a", testDeleteOpts()); err != nil {
		t.Fatalf("Delete docs/a: %v", err)
	}
	if err := s.Documents.Delete(ctx, "docs/b", testDeleteOpts()); err != nil {
		t.Fatalf("Delete docs/b: %v", err)
	}

	// Purge should remove 2 documents
	n, err := s.Documents.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 2 {
		t.Errorf("Purge() = %d, want 2", n)
	}

	// docs/c should still exist
	doc, err := s.Documents.Read(ctx, "docs/c")
	if err != nil {
		t.Errorf("Read(docs/c) error = %v", err)
	}
	if doc.Content != "content c" {
		t.Errorf("Content = %q, want %q", doc.Content, "content c")
	}

	// docs/a should be gone permanently
	_, err = s.Documents.Read(ctx, "docs/a")
	if err != documents.ErrNotFound {
		t.Errorf("Read(docs/a) error = %v, want ErrNotFound", err)
	}
}

func TestPurge_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Purge with nothing to purge
	n, err := s.Documents.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Purge() = %d, want 0", n)
	}
}

func TestPurge_AllVersions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create multiple versions
	s.Documents.Write(ctx, "docs/versioned", "v1", testWriteOpts())
	s.Documents.Write(ctx, "docs/versioned", "v2", testWriteOpts())
	s.Documents.Write(ctx, "docs/versioned", "v3", testWriteOpts())

	// Delete the document (soft-deletes all versions)
	if err := s.Documents.Delete(ctx, "docs/versioned", testDeleteOpts()); err != nil {
		t.Fatalf("Delete docs/versioned: %v", err)
	}

	// Purge should remove all 3 versions
	n, err := s.Documents.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 3 {
		t.Errorf("Purge() = %d, want 3", n)
	}
}
