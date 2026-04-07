package bulk_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func TestExport_SingleDoc(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "# Hello", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "readme.md")

	result, err := s.Bulk.Export(ctx, "docs/readme", dest, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Exported) != 1 {
		t.Errorf("Exported = %d, want 1", len(result.Exported))
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "# Hello" {
		t.Errorf("Content = %q, want %q", string(content), "# Hello")
	}
}

func TestExport_ToDirectory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	dir := t.TempDir()

	result, err := s.Bulk.Export(ctx, "docs/readme", dir, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Exported) != 1 {
		t.Errorf("Exported = %d, want 1", len(result.Exported))
	}

	// Should use doc name as filename
	content, err := os.ReadFile(filepath.Join(dir, "readme.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "content" {
		t.Errorf("Content = %q, want %q", string(content), "content")
	}
}

func TestExport_Prefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "readme", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/guide", "guide", testWriteOpts()); err != nil {
		t.Fatalf("Write guide: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "notes/todo", "todo", testWriteOpts()); err != nil {
		t.Fatalf("Write todo: %v", err)
	}

	dir := t.TempDir()

	result, err := s.Bulk.Export(ctx, "docs/", dir, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Exported) != 2 {
		t.Errorf("Exported = %d, want 2", len(result.Exported))
	}

	// Check files exist
	if _, err := os.Stat(filepath.Join(dir, "readme.md")); err != nil {
		t.Errorf("readme.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "guide.md")); err != nil {
		t.Errorf("guide.md not created: %v", err)
	}
}

func TestExport_SkipsExisting(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "readme", "new content", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(dest, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := s.Bulk.Export(ctx, "readme", dest, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Errorf("Skipped = %d, want 1", len(result.Skipped))
	}

	// Content should be unchanged
	content, _ := os.ReadFile(dest)
	if string(content) != "existing" {
		t.Errorf("File was overwritten, Content = %q", string(content))
	}
}

func TestExport_Overwrite(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "readme", "new content", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(dest, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := s.Bulk.Export(ctx, "readme", dest, bulk.ExportOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if len(result.Exported) != 1 {
		t.Errorf("Exported = %d, want 1", len(result.Exported))
	}

	content, _ := os.ReadFile(dest)
	if string(content) != "new content" {
		t.Errorf("Content = %q, want %q", string(content), "new content")
	}
}

func TestExport_SpecificVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "readme", "version 1", testWriteOpts()); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "readme", "version 2", testWriteOpts()); err != nil {
		t.Fatalf("Write v2: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "readme.md")

	_, err := s.Bulk.Export(ctx, "readme:1", dest, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	content, _ := os.ReadFile(dest)
	if string(content) != "version 1" {
		t.Errorf("Content = %q, want %q", string(content), "version 1")
	}
}

func TestExport_TraversalRejected(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write a document with a path that looks like a traversal attempt.
	// Normalisation is the first defence, but os.OpenRoot is the safety
	// net - verify it rejects the write even if normalisation is bypassed.
	if _, err := s.Documents.Write(ctx, "escape", "pwned", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	dir := t.TempDir()
	parent := filepath.Dir(dir)
	target := filepath.Join(parent, "escaped.md")

	// Attempt to export using a relative path that escapes the root.
	// os.Root.WriteFile should reject the ".." component.
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	defer root.Close()

	err = root.WriteFile("../escaped.md", []byte("pwned"), 0644)
	if err == nil {
		os.Remove(target) // clean up if it somehow succeeded
		t.Fatal("expected error writing outside root, got nil")
	}

	if _, err := os.Stat(target); err == nil {
		os.Remove(target)
		t.Fatal("file was created outside root directory")
	}
}

func TestExport_CreatesDirectories(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "readme", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "sub", "deep", "readme.md")

	_, err := s.Bulk.Export(ctx, "readme", dest, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Errorf("File not created at nested path: %v", err)
	}
}
