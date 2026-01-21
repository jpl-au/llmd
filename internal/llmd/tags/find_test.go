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
	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Documents.Write(ctx, "docs/c", "content c", testWriteOpts())
	s.Documents.Write(ctx, "notes/d", "content d", testWriteOpts())

	s.Tags.Add(ctx, "docs/a", "important", testOpts())
	s.Tags.Add(ctx, "docs/b", "important", testOpts())
	s.Tags.Add(ctx, "docs/c", "draft", testOpts())
	s.Tags.Add(ctx, "notes/d", "important", testOpts())

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

	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Documents.Write(ctx, "notes/c", "content c", testWriteOpts())

	s.Tags.Add(ctx, "docs/a", "important", testOpts())
	s.Tags.Add(ctx, "docs/b", "important", testOpts())
	s.Tags.Add(ctx, "notes/c", "important", testOpts())

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

	s.Documents.Write(ctx, "docs/a", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/a", "existing", testOpts())

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

	s.Documents.Write(ctx, "docs/a", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/a", "important", testOpts())
	s.Tags.Remove(ctx, "docs/a", "important", testOpts())

	paths, err := s.Tags.Find(ctx, "important")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("Find() should exclude removed tags, got %v", paths)
	}
}
