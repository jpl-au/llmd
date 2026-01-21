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

	if len(result.Created) != 1 {
		t.Errorf("Created = %d, want 1", len(result.Created))
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

	if len(result.Created) != 3 {
		t.Errorf("Created = %d, want 3", len(result.Created))
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

	if len(result.Created) != 1 {
		t.Errorf("Created = %d, want 1", len(result.Created))
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

	if len(result.Created) != 1 {
		t.Errorf("Created = %d, want 1 (hidden should be skipped)", len(result.Created))
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

	if len(result.Created) != 2 {
		t.Errorf("Created = %d, want 2", len(result.Created))
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

	if len(result.Created) != 1 {
		t.Errorf("Created = %d, want 1", len(result.Created))
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

	if len(result.Created) != 1 {
		t.Errorf("Created = %d, want 1 (only .md files)", len(result.Created))
	}
}

func TestImport_SkipsUnchanged(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	os.WriteFile(path, []byte("content"), 0644)

	// First import creates the document
	result1, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result1.Created) != 1 {
		t.Errorf("Created = %d, want 1", len(result1.Created))
	}

	// Second import with same content should skip
	result2, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result2.Skipped) != 1 {
		t.Errorf("Skipped = %d, want 1", len(result2.Skipped))
	}
	if len(result2.Created) != 0 {
		t.Errorf("Created = %d, want 0", len(result2.Created))
	}
	if len(result2.Updated) != 0 {
		t.Errorf("Updated = %d, want 0", len(result2.Updated))
	}
}

func TestImport_UpdatesChanged(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	os.WriteFile(path, []byte("version 1"), 0644)

	// First import creates the document
	result1, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result1.Created) != 1 {
		t.Errorf("Created = %d, want 1", len(result1.Created))
	}

	// Modify the file
	os.WriteFile(path, []byte("version 2"), 0644)

	// Second import should update
	result2, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result2.Updated) != 1 {
		t.Errorf("Updated = %d, want 1", len(result2.Updated))
	}
	if len(result2.Created) != 0 {
		t.Errorf("Created = %d, want 0", len(result2.Created))
	}

	// Verify content was updated
	doc, err := s.Documents.Read(ctx, "readme")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if doc.Content != "version 2" {
		t.Errorf("Content = %q, want %q", doc.Content, "version 2")
	}
	if doc.Version != 2 {
		t.Errorf("Version = %d, want 2", doc.Version)
	}
}

func TestImport_ForceBypassesSkip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "readme.md")
	os.WriteFile(path, []byte("content"), 0644)

	// First import creates the document
	_, err := s.Bulk.Import(ctx, dir, testImportOpts())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Second import with Force bypasses bulk's skip check
	// but Documents.Write still won't create duplicate version
	opts := testImportOpts()
	opts.Force = true
	result, err := s.Bulk.Import(ctx, dir, opts)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Force bypasses bulk-level skip, reports as Updated
	if len(result.Updated) != 1 {
		t.Errorf("Updated = %d, want 1", len(result.Updated))
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped = %d, want 0 (Force should bypass skip)", len(result.Skipped))
	}

	// Documents.Write deduplicates, so version stays at 1
	doc, err := s.Documents.Read(ctx, "readme")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1 (Documents.Write deduplicates)", doc.Version)
	}
}
