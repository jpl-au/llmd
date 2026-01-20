package tags_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func TestRemove(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/readme", "important", testOpts())

	err := s.Tags.Remove(ctx, "docs/readme", "important", testOpts())
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Tag should no longer appear in list
	list, _ := s.Tags.List(ctx, "docs/readme", testOpts())
	if len(list) != 0 {
		t.Errorf("List() after remove returned %d tags, want 0", len(list))
	}
}

func TestRemove_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	err := s.Tags.Remove(ctx, "docs/readme", "nonexistent", testOpts())
	if !errors.Is(err, tags.ErrNotFound) {
		t.Errorf("Remove() error = %v, want ErrNotFound", err)
	}
}

func TestRemove_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	doc, _ := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/readme", "important", testOpts())

	// Remove by document key
	err := s.Tags.Remove(ctx, doc.Key, "important", testOpts())
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	list, _ := s.Tags.List(ctx, "docs/readme", testOpts())
	if len(list) != 0 {
		t.Errorf("List() after remove returned %d tags, want 0", len(list))
	}
}
