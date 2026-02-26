package documents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestCopy(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/original", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.Copy(ctx, "docs/original", "docs/copy", testCopyOpts())
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	if doc.Content != "content" {
		t.Errorf("Content = %q, want %q", doc.Content, "content")
	}
	if doc.Path != "docs/copy" {
		t.Errorf("Path = %q, want %q", doc.Path, "docs/copy")
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1", doc.Version)
	}

	orig, err := s.Documents.Read(ctx, "docs/original")
	if err != nil {
		t.Fatalf("Read(original) error = %v", err)
	}
	if orig.Content != "content" {
		t.Errorf("Original content = %q, want %q", orig.Content, "content")
	}
}

func TestCopy_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Copy(ctx, "nonexistent", "docs/copy", testCopyOpts())
	if !errors.Is(err, documents.ErrNotFound) {
		t.Errorf("Copy() error = %v, want ErrNotFound", err)
	}
}
