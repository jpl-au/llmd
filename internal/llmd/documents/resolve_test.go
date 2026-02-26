package documents_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestResolve_ByPath(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.Resolve(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if doc.Path != "docs/readme" {
		t.Errorf("Path = %q, want %q", doc.Path, "docs/readme")
	}
	if doc.Content != "hello world" {
		t.Errorf("Content = %q, want %q", doc.Content, "hello world")
	}
}

func TestResolve_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, err := s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.Resolve(ctx, written.Key)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if doc.Key != written.Key {
		t.Errorf("Key = %q, want %q", doc.Key, written.Key)
	}
}

func TestResolve_ByFilesystem(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("filesystem content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	doc, err := s.Documents.Resolve(ctx, path)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if doc.Path != path {
		t.Errorf("Path = %q, want %q", doc.Path, path)
	}
	if doc.Content != "filesystem content" {
		t.Errorf("Content = %q, want %q", doc.Content, "filesystem content")
	}
	if doc.Source != "filesystem" {
		t.Errorf("Source = %q, want %q", doc.Source, "filesystem")
	}
}

func TestResolve_FilesystemPriority(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create a temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("filesystem content"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Also create a document with the same path in the store
	if _, err := s.Documents.Write(ctx, path, "store content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Filesystem should take priority
	doc, err := s.Documents.Resolve(ctx, path)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if doc.Source != "filesystem" {
		t.Errorf("Source = %q, want %q (filesystem should have priority)", doc.Source, "filesystem")
	}
	if doc.Content != "filesystem content" {
		t.Errorf("Content = %q, want %q", doc.Content, "filesystem content")
	}
}

func TestResolve_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Resolve(ctx, "nonexistent")
	if err != documents.ErrNotFound {
		t.Errorf("Resolve() error = %v, want ErrNotFound", err)
	}
}

func TestResolve_IgnoresDirectories(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()

	_, err := s.Documents.Resolve(ctx, dir)
	if err != documents.ErrNotFound {
		t.Errorf("Resolve(dir) error = %v, want ErrNotFound", err)
	}
}
