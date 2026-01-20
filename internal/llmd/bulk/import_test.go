package bulk_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestImport_SingleFile(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	os.WriteFile(path, []byte("# Hello"), 0644)

	result, err := s.Bulk.Import(ctx, path, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported = %d, want 1", len(result.Imported))
	}

	// Verify doc was created
	doc, err := s.Documents.Read(ctx, "readme")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if doc.Content != "# Hello" {
		t.Errorf("Content = %q, want %q", doc.Content, "# Hello")
	}
}

func TestImport_Directory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create temp directory with files
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("readme"), 0644)
	os.WriteFile(filepath.Join(dir, "guide.md"), []byte("guide"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "nested.md"), []byte("nested"), 0644)

	result, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 3 {
		t.Errorf("Imported = %d, want 3", len(result.Imported))
	}

	// Check nested file preserved path
	doc, err := s.Documents.Read(ctx, "sub/nested")
	if err != nil {
		t.Fatalf("Read(sub/nested) error = %v", err)
	}
	if doc.Content != "nested" {
		t.Errorf("Content = %q, want %q", doc.Content, "nested")
	}
}

func TestImport_WithPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("content"), 0644)

	opts := testImportOpts()
	opts.Prefix = "docs/"
	result, err := s.Bulk.Import(ctx, dir, opts)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported = %d, want 1", len(result.Imported))
	}

	// Verify prefix was applied
	_, err = s.Documents.Read(ctx, "docs/readme")
	if err != nil {
		t.Errorf("Read(docs/readme) error = %v", err)
	}
}

func TestImport_Flatten(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "deep", "file.md"), []byte("content"), 0644)

	opts := testImportOpts()
	opts.Flatten = true
	_, err := s.Bulk.Import(ctx, dir, opts)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Should be flattened to just "file"
	_, err = s.Documents.Read(ctx, "file")
	if err != nil {
		t.Errorf("Read(file) error = %v", err)
	}
}

func TestImport_SkipsHidden(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.md"), []byte("visible"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("hidden"), 0644)

	result, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported = %d, want 1 (hidden should be skipped)", len(result.Imported))
	}
}

func TestImport_IncludesHidden(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.md"), []byte("visible"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte("hidden"), 0644)

	opts := testImportOpts()
	opts.Hidden = true
	result, err := s.Bulk.Import(ctx, dir, opts)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 2 {
		t.Errorf("Imported = %d, want 2", len(result.Imported))
	}
}

func TestImport_DryRun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("content"), 0644)

	opts := testImportOpts()
	opts.DryRun = true
	result, err := s.Bulk.Import(ctx, dir, opts)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported = %d, want 1", len(result.Imported))
	}

	// Verify nothing was actually written
	_, err = s.Documents.Read(ctx, "readme")
	if err != documents.ErrNotFound {
		t.Errorf("DryRun should not create documents")
	}
}

func TestImport_OnlyMarkdown(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("markdown"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.json"), []byte("{}"), 0644)

	result, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported = %d, want 1 (only .md files)", len(result.Imported))
	}
}
