package search_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/pkg/model/core"
)

func TestFTSHandler_Write(t *testing.T) {
	store := ftsTestStore(t)
	ctx := context.Background()

	// Write a document
	doc, err := store.Documents.Write(ctx, "test/fts", "hello world", ftsWriteOpts())
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Search for it
	results, err := store.Search.FullText(ctx, "hello")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Key != doc.Key {
		t.Errorf("expected key %s, got %s", doc.Key, results[0].Key)
	}
}

func TestFTSHandler_WriteUpdate(t *testing.T) {
	store := ftsTestStore(t)
	ctx := context.Background()

	// Write initial version
	_, err := store.Documents.Write(ctx, "test/update", "original content", ftsWriteOpts())
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Update with new content
	doc2, err := store.Documents.Write(ctx, "test/update", "updated content", ftsWriteOpts())
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	// Search for original - should not find
	results, err := store.Search.FullText(ctx, "original")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'original', got %d", len(results))
	}

	// Search for updated - should find
	results, err = store.Search.FullText(ctx, "updated")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'updated', got %d", len(results))
	}
	if len(results) > 0 && results[0].Key != doc2.Key {
		t.Errorf("expected key %s, got %s", doc2.Key, results[0].Key)
	}
}

func TestFTSHandler_Delete(t *testing.T) {
	store := ftsTestStore(t)
	ctx := context.Background()

	// Write a document
	_, err := store.Documents.Write(ctx, "test/delete", "searchable content", ftsWriteOpts())
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify it's searchable
	results, err := store.Search.FullText(ctx, "searchable")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	// Delete the document
	err = store.Documents.Delete(ctx, "test/delete", ftsDeleteOpts())
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify it's no longer searchable
	results, err = store.Search.FullText(ctx, "searchable")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestFTSHandler_Restore(t *testing.T) {
	store := ftsTestStore(t)
	ctx := context.Background()

	// Write a document
	_, err := store.Documents.Write(ctx, "test/restore", "restorable content", ftsWriteOpts())
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Delete it
	err = store.Documents.Delete(ctx, "test/restore", ftsDeleteOpts())
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify not searchable
	results, err := store.Search.FullText(ctx, "restorable")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(results))
	}

	// Restore it
	err = store.Documents.Restore(ctx, "test/restore", ftsRestoreOpts())
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify searchable again
	results, err = store.Search.FullText(ctx, "restorable")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result after restore, got %d", len(results))
	}
}

func TestFTSHandler_Move(t *testing.T) {
	store := ftsTestStore(t)
	ctx := context.Background()

	// Write a document
	_, err := store.Documents.Write(ctx, "old/path", "moveable content", ftsWriteOpts())
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Move it
	err = store.Documents.Move(ctx, "old/path", "new/path", ftsMoveOpts())
	if err != nil {
		t.Fatalf("move failed: %v", err)
	}

	// Search should still find it
	results, err := store.Search.FullText(ctx, "moveable")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after move, got %d", len(results))
	}

	// Verify it has the new path
	if results[0].Path != "new/path" {
		t.Errorf("expected path new/path, got %s", results[0].Path)
	}
}

// Test helpers
func ftsTestStore(t *testing.T) *llmd.Store {
	t.Helper()
	s, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ftsWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func ftsDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func ftsRestoreOpts() documents.RestoreOptions {
	return documents.RestoreOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}

func ftsMoveOpts() documents.MoveOptions {
	return documents.MoveOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}
