package tags_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func TestFind(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create documents and add tags
	if _, err := s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts()); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts()); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/c", "content c", testWriteOpts()); err != nil {
		t.Fatalf("Write c: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "notes/d", "content d", testWriteOpts()); err != nil {
		t.Fatalf("Write d: %v", err)
	}

	if _, err := s.Tags.Add(ctx, "docs/a", "important", testOpts()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/b", "important", testOpts()); err != nil {
		t.Fatalf("Add(docs/b important): %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/c", "draft", testOpts()); err != nil {
		t.Fatalf("Add(docs/c draft): %v", err)
	}
	if _, err := s.Tags.Add(ctx, "notes/d", "important", testOpts()); err != nil {
		t.Fatalf("Add(notes/d important): %v", err)
	}

	paths, err := s.Tags.Find(ctx, "important")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(paths) != 3 {
		t.Errorf("Find() returned %d paths, want 3", len(paths))
	}

	if !slices.Contains(paths, "docs/a") || !slices.Contains(paths, "docs/b") || !slices.Contains(paths, "notes/d") {
		t.Errorf("Find() = %v, want [docs/a, docs/b, notes/d]", paths)
	}
}

func TestFind_WithRelationPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts()); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts()); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "notes/c", "content c", testWriteOpts()); err != nil {
		t.Fatalf("Write c: %v", err)
	}

	if _, err := s.Tags.Add(ctx, "docs/a", "important", testOpts()); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/b", "important", testOpts()); err != nil {
		t.Fatalf("Add b: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "notes/c", "important", testOpts()); err != nil {
		t.Fatalf("Add c: %v", err)
	}

	paths, err := s.Tags.Find(ctx, "important", tags.FindOptions{RelationPrefix: "docs/"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(paths) != 2 {
		t.Errorf("Find(prefix=docs/) returned %d paths, want 2", len(paths))
	}

	if slices.Contains(paths, "notes/c") {
		t.Errorf("Find(prefix=docs/) should not include notes/c")
	}
}

func TestFind_NoResults(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/a", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/a", "existing", testOpts()); err != nil {
		t.Fatalf("Add: %v", err)
	}

	paths, err := s.Tags.Find(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("Find(nonexistent) returned %d paths, want 0", len(paths))
	}
}

func TestFind_ExcludesDeleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/a", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/a", "important", testOpts()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Tags.Remove(ctx, "docs/a", "important", testOpts()); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	paths, err := s.Tags.Find(ctx, "important")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("Find() should exclude removed tags, got %v", paths)
	}
}
